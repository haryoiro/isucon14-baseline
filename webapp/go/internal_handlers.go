package main

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type matchRide struct {
	ID      string `db:"id"`
	PickLat int    `db:"pickup_latitude"`
	PickLon int    `db:"pickup_longitude"`
	DestLat int    `db:"destination_latitude"`
	DestLon int    `db:"destination_longitude"`
}

var matchMu sync.Mutex

func startMatcher() {
	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			if err := runMatching(context.Background()); err != nil {
				slog.Error("matching", "err", err)
			}
		}
	}()
}

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching(w http.ResponseWriter, r *http.Request) {
	if err := runMatching(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func runMatching(ctx context.Context) error {
	matchMu.Lock()
	defer matchMu.Unlock()
	rides := []matchRide{}
	if err := db.SelectContext(ctx, &rides, `SELECT id, pickup_latitude, pickup_longitude, destination_latitude, destination_longitude FROM rides WHERE chair_id IS NULL ORDER BY created_at`); err != nil {
		return err
	}
	if len(rides) == 0 {
		return nil
	}
	type assign struct {
		ride  matchRide
		chair *chairState
	}
	assigns := []assign{}
	stMu.Lock()
	free := []*chairState{}
	for _, c := range chairStates {
		if c.Active && c.RideID == "" && c.HasLoc {
			free = append(free, c)
		}
	}
	for _, ride := range rides {
		best := -1
		bestScore := 0.0
		for i, c := range free {
			if c == nil {
				continue
			}
			d := calculateDistance(c.Lat, c.Lon, ride.PickLat, ride.PickLon)
			score := float64(d) / float64(max(c.Speed, 1))
			if best < 0 || score < bestScore {
				best = i
				bestScore = score
			}
		}
		if best < 0 {
			break
		}
		c := free[best]
		free[best] = nil
		c.RideID = ride.ID
		c.RideStatus = "MATCHING"
		c.PickLat, c.PickLon, c.DestLat, c.DestLon = ride.PickLat, ride.PickLon, ride.DestLat, ride.DestLon
		assigns = append(assigns, assign{ride, c})
	}
	stMu.Unlock()

	for _, a := range assigns {
		res, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ? AND chair_id IS NULL", a.chair.ID, a.ride.ID)
		ok := err == nil
		if ok {
			n, _ := res.RowsAffected()
			ok = n == 1
		}
		if !ok {
			stMu.Lock()
			if a.chair.RideID == a.ride.ID {
				a.chair.RideID = ""
				a.chair.RideStatus = ""
			}
			stMu.Unlock()
			if err != nil {
				return err
			}
			continue
		}
		publishRide(ctx, a.ride.ID, "MATCHING", false, true)
	}
	return nil
}
