package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
)

type hub struct {
	mu   sync.Mutex
	subs map[string]map[chan []byte]struct{}
}

func newHub() *hub { return &hub{subs: map[string]map[chan []byte]struct{}{}} }

func (h *hub) sub(id string) chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	m := h.subs[id]
	if m == nil {
		m = map[chan []byte]struct{}{}
		h.subs[id] = m
	}
	m[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *hub) unsub(id string, ch chan []byte) {
	h.mu.Lock()
	if m := h.subs[id]; m != nil {
		delete(m, ch)
		if len(m) == 0 {
			delete(h.subs, id)
		}
	}
	h.mu.Unlock()
}

func (h *hub) pub(id string, msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[id] {
		select {
		case ch <- msg:
		default:
			slog.Error("notification dropped", "id", id)
		}
	}
}

var (
	appHub   = newHub()
	chairHub = newHub()
	// COMPLETED を椅子へ通知し終えるまでマッチング対象から外す椅子
	pendingComplete sync.Map
)

var statusIndex = map[string]int{"MATCHING": 0, "ENROUTE": 1, "PICKUP": 2, "CARRYING": 3, "ARRIVED": 4, "COMPLETED": 5}

type notifyEnvelope struct {
	RideID string
	Status string
	Body   []byte
}

func buildAppNotification(ctx context.Context, ride *Ride, status string) ([]byte, error) {
	fare, err := calculateDiscountedFare(ctx, db, ride.UserID, ride, 0, 0, 0, 0)
	if err != nil {
		return nil, err
	}
	data := &appGetNotificationResponseData{
		RideID:                ride.ID,
		PickupCoordinate:      Coordinate{Latitude: ride.PickupLatitude, Longitude: ride.PickupLongitude},
		DestinationCoordinate: Coordinate{Latitude: ride.DestinationLatitude, Longitude: ride.DestinationLongitude},
		Fare:                  fare,
		Status:                status,
		CreatedAt:             ride.CreatedAt.UnixMilli(),
		UpdateAt:              ride.UpdatedAt.UnixMilli(),
	}
	if ride.ChairID.Valid {
		chair, err := getChairByID(ctx, ride.ChairID.String)
		if err != nil {
			return nil, err
		}
		stats, err := getChairStats(ctx, db, chair.ID)
		if err != nil {
			return nil, err
		}
		data.Chair = &appGetNotificationResponseChair{ID: chair.ID, Name: chair.Name, Model: chair.Model, Stats: stats}
	}
	return json.Marshal(data)
}

func buildChairNotification(ctx context.Context, ride *Ride, status string) ([]byte, error) {
	user, err := getUserByID(ctx, ride.UserID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(&chairGetNotificationResponseData{
		RideID:                ride.ID,
		User:                  simpleUser{ID: user.ID, Name: fmt.Sprintf("%s %s", user.Firstname, user.Lastname)},
		PickupCoordinate:      Coordinate{Latitude: ride.PickupLatitude, Longitude: ride.PickupLongitude},
		DestinationCoordinate: Coordinate{Latitude: ride.DestinationLatitude, Longitude: ride.DestinationLongitude},
		Status:                status,
	})
}

// publishRide はライドの状態変化をユーザーと（割り当て済みなら）椅子へ通知する
func publishRide(ctx context.Context, rideID string, status string, toApp, toChair bool) {
	ctx = context.WithoutCancel(ctx)
	ride := &Ride{}
	if err := db.GetContext(ctx, ride, `SELECT * FROM rides WHERE id = ?`, rideID); err != nil {
		slog.Error("publish: ride", "err", err)
		return
	}
	if toApp {
		b, err := buildAppNotification(ctx, ride, status)
		if err != nil {
			slog.Error("publish: app", "err", err)
		} else {
			appHub.pub(ride.UserID, encodeEnvelope(ride.ID, status, b))
		}
	}
	if toChair && ride.ChairID.Valid {
		b, err := buildChairNotification(ctx, ride, status)
		if err != nil {
			slog.Error("publish: chair", "err", err)
		} else {
			chairHub.pub(ride.ChairID.String, encodeEnvelope(ride.ID, status, b))
		}
	}
}

// 先頭 26 バイト = ride_id, 次の 1 バイト = status index, 残り = JSON
func encodeEnvelope(rideID, status string, body []byte) []byte {
	b := make([]byte, 0, 27+len(body))
	b = append(b, rideID...)
	b = append(b, byte(statusIndex[status]))
	b = append(b, body...)
	return b
}

func serveSSE(w http.ResponseWriter, r *http.Request, h *hub, id string, initial func(ctx context.Context) ([]byte, error)) {
	ctx := r.Context()
	ch := h.sub(id)
	defer h.unsub(id, ch)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	lastRide := ""
	lastIdx := -1
	past := map[string]struct{}{}
	write := func(env []byte) bool {
		if len(env) < 27 {
			return true
		}
		rid := string(env[:26])
		idx := int(env[26])
		if rid == lastRide && idx <= lastIdx {
			return true
		}
		if _, ok := past[rid]; ok {
			return true
		}
		if rid != lastRide && lastRide != "" {
			past[lastRide] = struct{}{}
		}
		lastRide, lastIdx = rid, idx
		if _, err := w.Write([]byte("data: ")); err != nil {
			return false
		}
		w.Write(env[27:])
		if _, err := w.Write([]byte("\n\n")); err != nil {
			return false
		}
		if flusher != nil {
			flusher.Flush()
		}
		return true
	}

	init, err := initial(ctx)
	if err != nil {
		slog.Error("sse initial", "err", err)
		return
	}
	if init != nil {
		if !write(init) {
			return
		}
	} else if flusher != nil {
		flusher.Flush()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case env := <-ch:
			if !write(env) {
				return
			}
		}
	}
}

func appGetNotification(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*User)
	serveSSE(w, r, appHub, user.ID, func(ctx context.Context) ([]byte, error) {
		ride := &Ride{}
		if err := db.GetContext(ctx, ride, `SELECT * FROM rides WHERE user_id = ? ORDER BY created_at DESC LIMIT 1`, user.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}
		status, err := getLatestRideStatus(ctx, db, ride.ID)
		if err != nil {
			return nil, err
		}
		b, err := buildAppNotification(ctx, ride, status)
		if err != nil {
			return nil, err
		}
		return encodeEnvelope(ride.ID, status, b), nil
	})
}

func chairGetNotification(w http.ResponseWriter, r *http.Request) {
	chair := r.Context().Value("chair").(*Chair)
	serveSSE(w, r, chairHub, chair.ID, func(ctx context.Context) ([]byte, error) {
		ride := &Ride{}
		if err := db.GetContext(ctx, ride, `SELECT * FROM rides WHERE chair_id = ? ORDER BY updated_at DESC LIMIT 1`, chair.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}
		status, err := getLatestRideStatus(ctx, db, ride.ID)
		if err != nil {
			return nil, err
		}
		b, err := buildChairNotification(ctx, ride, status)
		if err != nil {
			return nil, err
		}
		return encodeEnvelope(ride.ID, status, b), nil
	})
}

// キャッシュ（不変なデータのみ）
var (
	chairByID   sync.Map
	chairByTok  sync.Map
	userByID    sync.Map
	userByTok   sync.Map
	ownerByTok  sync.Map
)

func getChairByID(ctx context.Context, id string) (*Chair, error) {
	if v, ok := chairByID.Load(id); ok {
		return v.(*Chair), nil
	}
	c := &Chair{}
	if err := db.GetContext(ctx, c, `SELECT * FROM chairs WHERE id = ?`, id); err != nil {
		return nil, err
	}
	chairByID.Store(id, c)
	return c, nil
}

func getUserByID(ctx context.Context, id string) (*User, error) {
	if v, ok := userByID.Load(id); ok {
		return v.(*User), nil
	}
	u := &User{}
	if err := db.GetContext(ctx, u, `SELECT * FROM users WHERE id = ?`, id); err != nil {
		return nil, err
	}
	userByID.Store(id, u)
	return u, nil
}

func clearCaches() {
	for _, m := range []*sync.Map{&chairByID, &chairByTok, &userByID, &userByTok, &ownerByTok, &pendingComplete} {
		m.Range(func(k, _ any) bool { m.Delete(k); return true })
	}
}
