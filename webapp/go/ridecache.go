package main

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

type rideInfo struct {
	Ride   Ride
	Fare   int
	Status string
}

type chairStat struct {
	Count int
	Sum   int
}

var (
	rcMu       sync.Mutex
	rideCache  = map[string]*rideInfo{}
	chairStats = map[string]*chairStat{}
)

func loadRideCache(ctx context.Context) error {
	type row struct {
		Ride
		Discount int `db:"discount"`
	}
	rows := []row{}
	if err := db.SelectContext(ctx, &rows, `SELECT r.*, IFNULL(c.discount, 0) AS discount FROM rides r LEFT JOIN coupons c ON c.used_by = r.id`); err != nil {
		return err
	}
	type st struct {
		RideID string `db:"ride_id"`
		Status string `db:"status"`
	}
	sts := []st{}
	if err := db.SelectContext(ctx, &sts, `SELECT ride_id, status FROM ride_statuses ORDER BY created_at`); err != nil {
		return err
	}
	rc := make(map[string]*rideInfo, len(rows))
	cs := map[string]*chairStat{}
	for _, r := range rows {
		metered := farePerDistance * calculateDistance(r.PickupLatitude, r.PickupLongitude, r.DestinationLatitude, r.DestinationLongitude)
		rc[r.ID] = &rideInfo{Ride: r.Ride, Fare: initialFare + max(metered-r.Discount, 0)}
		if r.Evaluation != nil && r.ChairID.Valid {
			s := cs[r.ChairID.String]
			if s == nil {
				s = &chairStat{}
				cs[r.ChairID.String] = s
			}
			s.Count++
			s.Sum += *r.Evaluation
		}
	}
	for _, s := range sts {
		if ri := rc[s.RideID]; ri != nil {
			ri.Status = s.Status
		}
	}
	rcMu.Lock()
	rideCache = rc
	chairStats = cs
	rcMu.Unlock()
	return nil
}

func cacheNewRide(r Ride, fare int) {
	rcMu.Lock()
	rideCache[r.ID] = &rideInfo{Ride: r, Fare: fare, Status: "MATCHING"}
	rcMu.Unlock()
}

func cacheAssign(rideID, chairID string, at time.Time) {
	rcMu.Lock()
	if ri := rideCache[rideID]; ri != nil {
		ri.Ride.ChairID = sql.NullString{String: chairID, Valid: true}
		ri.Ride.UpdatedAt = at
	}
	rcMu.Unlock()
}

func cacheStatus(rideID, status string) {
	rcMu.Lock()
	if ri := rideCache[rideID]; ri != nil {
		ri.Status = status
	}
	rcMu.Unlock()
}

func cacheComplete(r Ride) {
	rcMu.Lock()
	if ri := rideCache[r.ID]; ri != nil {
		ri.Ride = r
		ri.Status = "COMPLETED"
	}
	if r.ChairID.Valid && r.Evaluation != nil {
		s := chairStats[r.ChairID.String]
		if s == nil {
			s = &chairStat{}
			chairStats[r.ChairID.String] = s
		}
		s.Count++
		s.Sum += *r.Evaluation
	}
	rcMu.Unlock()
}

// 返り値はコピー
func getRideInfo(rideID string) (rideInfo, bool) {
	rcMu.Lock()
	defer rcMu.Unlock()
	ri := rideCache[rideID]
	if ri == nil {
		return rideInfo{}, false
	}
	return *ri, true
}

func getChairStatsMem(chairID string) appGetNotificationResponseChairStats {
	rcMu.Lock()
	defer rcMu.Unlock()
	s := chairStats[chairID]
	if s == nil || s.Count == 0 {
		return appGetNotificationResponseChairStats{}
	}
	return appGetNotificationResponseChairStats{TotalRidesCount: s.Count, TotalEvaluationAvg: float64(s.Sum) / float64(s.Count)}
}
