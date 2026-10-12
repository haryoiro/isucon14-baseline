package main

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type chairState struct {
	ID        string
	OwnerID   string
	Name      string
	Model     string
	Speed     int
	Active    bool
	CreatedAt time.Time

	HasLoc       bool
	Lat, Lon     int
	TotalDist    int
	LocUpdatedAt time.Time

	// 現在のライド（COMPLETED を椅子へ通知し終えるまで保持）
	MatchNotified                      bool
	FreedAt                            time.Time
	RideID                             string
	RideStatus                         string
	PickLat, PickLon, DestLat, DestLon int
}

var (
	stMu        sync.Mutex
	chairStates = map[string]*chairState{}
	modelSpeed  = map[string]int{}

	locBufMu sync.Mutex
	locBuf   []locRow
	dirty    = map[string]struct{}{}
)

type locRow struct {
	ID       string
	ChairID  string
	Lat, Lon int
	At       time.Time
}

func loadState(ctx context.Context) error {
	if err := loadRideCache(ctx); err != nil {
		return err
	}
	type row struct {
		ID        string    `db:"id"`
		OwnerID   string    `db:"owner_id"`
		Name      string    `db:"name"`
		Model     string    `db:"model"`
		IsActive  bool      `db:"is_active"`
		CreatedAt time.Time `db:"created_at"`
	}
	models := []ChairModel{}
	if err := db.SelectContext(ctx, &models, `SELECT * FROM chair_models`); err != nil {
		return err
	}
	rows := []row{}
	if err := db.SelectContext(ctx, &rows, `SELECT id, owner_id, name, model, is_active, created_at FROM chairs`); err != nil {
		return err
	}
	type tot struct {
		ChairID   string    `db:"chair_id"`
		Total     int       `db:"total_distance"`
		UpdatedAt time.Time `db:"total_distance_updated_at"`
		Lat       int       `db:"latitude"`
		Lon       int       `db:"longitude"`
	}
	tots := []tot{}
	if err := db.SelectContext(ctx, &tots, `SELECT * FROM chair_totals`); err != nil {
		return err
	}
	rides := []Ride{}
	if err := db.SelectContext(ctx, &rides, `SELECT * FROM rides WHERE chair_id IS NOT NULL AND evaluation IS NULL`); err != nil {
		return err
	}
	statuses := map[string]string{}
	for _, r := range rides {
		s, err := getLatestRideStatus(ctx, db, r.ID)
		if err != nil {
			return err
		}
		statuses[r.ID] = s
	}

	stMu.Lock()
	defer stMu.Unlock()
	modelSpeed = map[string]int{}
	for _, m := range models {
		modelSpeed[m.Name] = m.Speed
	}
	chairStates = map[string]*chairState{}
	for _, r := range rows {
		chairStates[r.ID] = &chairState{ID: r.ID, OwnerID: r.OwnerID, Name: r.Name, Model: r.Model, Speed: modelSpeed[r.Model], Active: r.IsActive, CreatedAt: r.CreatedAt}
	}
	for _, t := range tots {
		if c := chairStates[t.ChairID]; c != nil {
			c.HasLoc = true
			c.Lat, c.Lon, c.TotalDist, c.LocUpdatedAt = t.Lat, t.Lon, t.Total, t.UpdatedAt
		}
	}
	for _, r := range rides {
		if c := chairStates[r.ChairID.String]; c != nil {
			c.RideID = r.ID
			c.RideStatus = statuses[r.ID]
			c.PickLat, c.PickLon, c.DestLat, c.DestLon = r.PickupLatitude, r.PickupLongitude, r.DestinationLatitude, r.DestinationLongitude
		}
	}
	return nil
}

func resetLocBuf() {
	locBufMu.Lock()
	locBuf = nil
	dirty = map[string]struct{}{}
	locBufMu.Unlock()
}

func startFlusher() {
	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			flushLocations()
		}
	}()
}

var flushMu sync.Mutex

func flushLocations() {
	flushMu.Lock()
	defer flushMu.Unlock()
	locBufMu.Lock()
	buf := locBuf
	d := dirty
	locBuf = nil
	dirty = map[string]struct{}{}
	locBufMu.Unlock()
	if len(buf) == 0 {
		return
	}
	ctx := context.Background()
	for i := 0; i < len(buf); i += 500 {
		end := min(i+500, len(buf))
		var sb strings.Builder
		args := make([]any, 0, (end-i)*5)
		sb.WriteString("INSERT INTO chair_locations (id, chair_id, latitude, longitude, created_at) VALUES ")
		for j, r := range buf[i:end] {
			if j > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?)")
			args = append(args, r.ID, r.ChairID, r.Lat, r.Lon, r.At)
		}
		if _, err := db.ExecContext(ctx, sb.String(), args...); err != nil {
			slog.Error("flush chair_locations", "err", err)
		}
	}
	var sb strings.Builder
	args := []any{}
	sb.WriteString("INSERT INTO chair_totals (chair_id, total_distance, total_distance_updated_at, latitude, longitude) VALUES ")
	n := 0
	stMu.Lock()
	for id := range d {
		c := chairStates[id]
		if c == nil || !c.HasLoc {
			continue
		}
		if n > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("(?,?,?,?,?)")
		args = append(args, c.ID, c.TotalDist, c.LocUpdatedAt, c.Lat, c.Lon)
		n++
	}
	stMu.Unlock()
	if n == 0 {
		return
	}
	sb.WriteString(" ON DUPLICATE KEY UPDATE total_distance = VALUES(total_distance), total_distance_updated_at = VALUES(total_distance_updated_at), latitude = VALUES(latitude), longitude = VALUES(longitude)")
	if _, err := db.ExecContext(ctx, sb.String(), args...); err != nil {
		slog.Error("flush chair_totals", "err", err)
	}
}
