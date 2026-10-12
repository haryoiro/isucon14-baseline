package main

import (
	"net/http"
)

type matchRide struct {
	ID        string `db:"id"`
	Latitude  int    `db:"pickup_latitude"`
	Longitude int    `db:"pickup_longitude"`
}

type matchChair struct {
	ID        string `db:"id"`
	Speed     int    `db:"speed"`
	Latitude  int    `db:"latitude"`
	Longitude int    `db:"longitude"`
}

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rides := []matchRide{}
	if err := db.SelectContext(ctx, &rides, `SELECT id, pickup_latitude, pickup_longitude FROM rides WHERE chair_id IS NULL ORDER BY created_at`); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(rides) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	// 空いている椅子: アクティブで、割り当て済みライドがすべて COMPLETED を椅子に通知済み
	chairs := []matchChair{}
	if err := db.SelectContext(ctx, &chairs, `SELECT c.id, m.speed, t.latitude, t.longitude
		FROM chairs c JOIN chair_models m ON m.name = c.model JOIN chair_totals t ON t.chair_id = c.id
		WHERE c.is_active = TRUE AND NOT EXISTS (
			SELECT 1 FROM rides r WHERE r.chair_id = c.id AND r.evaluation IS NULL)`); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	used := make([]bool, len(chairs))
	for _, ride := range rides {
		best := -1
		bestScore := 0.0
		for i, c := range chairs {
			if used[i] {
				continue
			}
			if _, ok := pendingComplete.Load(c.ID); ok {
				continue
			}
			d := calculateDistance(c.Latitude, c.Longitude, ride.Latitude, ride.Longitude)
			score := float64(d) / float64(c.Speed)
			if best < 0 || score < bestScore {
				best = i
				bestScore = score
			}
		}
		if best < 0 {
			break
		}
		res, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ? AND chair_id IS NULL", chairs[best].ID, ride.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if n, _ := res.RowsAffected(); n == 1 {
			used[best] = true
			publishRide(ctx, ride.ID, "MATCHING", false, true)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
