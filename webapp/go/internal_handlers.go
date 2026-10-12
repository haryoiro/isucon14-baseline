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
	ID        string    `db:"id"`
	PickLat   int       `db:"pickup_latitude"`
	PickLon   int       `db:"pickup_longitude"`
	DestLat   int       `db:"destination_latitude"`
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
			time.Sleep(50 * time.Millisecond)
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
	rides := getWaitingRides()
	if len(rides) == 0 {
		return nil
	}
	type assign struct {
		ride  matchRide
		chair *chairState
	}
	type freeChair struct {
		p               *chairState
		lat, lon, speed int
	}
	stMu.Lock()
	free := []freeChair{}
	for _, c := range chairStates {
		if c.Active && c.RideID == "" && c.HasLoc {
			free = append(free, freeChair{c, c.Lat, c.Lon, max(c.Speed, 1)})
		}
	}
	stMu.Unlock()
	if len(free) == 0 {
		return nil
	}

	chosen := make([][2]int, 0, len(free))
	rideUsed := make([]bool, len(rides))
	chairUsed := make([]bool, len(free))
	now := time.Now()
	// 長く待っているライドは古い順に最寄りの椅子を割り当てる
	nFree := len(free)
	for ri, ride := range rides {
		if nFree == 0 || now.Sub(ride.CreatedAt) < urgentWait {
			break
		}
		best := -1
		bestScore := 0.0
		for ci, c := range free {
			if chairUsed[ci] {
				continue
			}
			score := float64(calculateDistance(c.lat, c.lon, ride.PickLat, ride.PickLon)) / float64(c.speed)
			if best < 0 || score < bestScore {
				best, bestScore = ci, score
			}
		}
		rideUsed[ri], chairUsed[best] = true, true
		nFree--
		chosen = append(chosen, [2]int{ri, best})
	}
	if nFree > 0 {
		type pair struct {
			r, c  int
			score float64
		}
		pairs := make([]pair, 0, 1024)
		for ri, ride := range rides {
			if rideUsed[ri] {
				continue
			}
			for ci, c := range free {
				if chairUsed[ci] {
					continue
				}
				pairs = append(pairs, pair{ri, ci, float64(calculateDistance(c.lat, c.lon, ride.PickLat, ride.PickLon)) / float64(c.speed)})
			}
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].score < pairs[j].score })
		for _, p := range pairs {
			if nFree == 0 {
				break
			}
			if rideUsed[p.r] || chairUsed[p.c] {
				continue
			}
			rideUsed[p.r], chairUsed[p.c] = true, true
			nFree--
			chosen = append(chosen, [2]int{p.r, p.c})
		}
	}

	assigns := make([]assign, 0, len(chosen))
	stMu.Lock()
	for _, rc := range chosen {
		ride := rides[rc[0]]
		c := free[rc[1]].p
		if !c.Active || c.RideID != "" {
			continue
		}
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
