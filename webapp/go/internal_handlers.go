package main

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"
)

type matchRide struct {
	ID      string `db:"id"`
	PickLat int    `db:"pickup_latitude"`
	PickLon int    `db:"pickup_longitude"`
	DestLat int    `db:"destination_latitude"`
	DestLon   int       `db:"destination_longitude"`
	CreatedAt time.Time `db:"created_at"`
}

var (
	matchMu      sync.Mutex
	lastMatchLog time.Time
)

const urgentWait = 3 * time.Second

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
	if err := db.SelectContext(ctx, &rides, `SELECT id, pickup_latitude, pickup_longitude, destination_latitude, destination_longitude, created_at FROM rides WHERE chair_id IS NULL ORDER BY created_at`); err != nil {
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
	type pair struct {
		r, c  int
		score float64
	}
	pairs := make([]pair, 0, len(rides)*len(free))
	for ri, ride := range rides {
		for ci, c := range free {
			d := calculateDistance(c.Lat, c.Lon, ride.PickLat, ride.PickLon)
			pairs = append(pairs, pair{ri, ci, float64(d) / float64(max(c.Speed, 1))})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].score < pairs[j].score })
	if time.Since(lastMatchLog) > time.Second {
		lastMatchLog = time.Now()
		active := 0
		for _, c := range chairStates {
			if c.Active {
				active++
			}
		}
		slog.Info("matching", "rides", len(rides), "free", len(free), "active", active, "chairs", len(chairStates))
	}
	rideUsed := make([]bool, len(rides))
	chairUsed := make([]bool, len(free))
	now := time.Now()
	// 長く待っているライドは古い順に最寄りの椅子を割り当てる
	for ri, ride := range rides {
		if now.Sub(ride.CreatedAt) < urgentWait {
			break
		}
		best := -1
		bestScore := 0.0
		for ci, c := range free {
			if chairUsed[ci] {
				continue
			}
			d := calculateDistance(c.Lat, c.Lon, ride.PickLat, ride.PickLon)
			score := float64(d) / float64(max(c.Speed, 1))
			if best < 0 || score < bestScore {
				best, bestScore = ci, score
			}
		}
		if best < 0 {
			break
		}
		rideUsed[ri], chairUsed[best] = true, true
		c := free[best]
		c.RideID = ride.ID
		c.RideStatus = "MATCHING"
		c.MatchNotified = false
		c.PickLat, c.PickLon, c.DestLat, c.DestLon = ride.PickLat, ride.PickLon, ride.DestLat, ride.DestLon
		assigns = append(assigns, assign{ride, c})
	}
	for _, p := range pairs {
		if rideUsed[p.r] || chairUsed[p.c] {
			continue
		}
		rideUsed[p.r], chairUsed[p.c] = true, true
		ride := rides[p.r]
		c := free[p.c]
		c.RideID = ride.ID
		c.RideStatus = "MATCHING"
		c.MatchNotified = false
		c.PickLat, c.PickLon, c.DestLat, c.DestLon = ride.PickLat, ride.PickLon, ride.DestLat, ride.DestLon
		assigns = append(assigns, assign{ride, c})
	}
	stMu.Unlock()

	for _, a := range assigns {
		at := time.Now().Truncate(time.Microsecond)
		res, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ?, updated_at = ? WHERE id = ? AND chair_id IS NULL", a.chair.ID, at, a.ride.ID)
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
		cacheAssign(a.ride.ID, a.chair.ID, at)
		publishRide(ctx, a.ride.ID, "MATCHING", false, true)
		stMu.Lock()
		if a.chair.RideID == a.ride.ID {
			a.chair.MatchNotified = true
		}
		stMu.Unlock()
	}
	return nil
}
