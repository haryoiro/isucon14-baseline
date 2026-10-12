package main

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"
)

type chairPostChairsRequest struct {
	Name               string `json:"name"`
	Model              string `json:"model"`
	ChairRegisterToken string `json:"chair_register_token"`
}

type chairPostChairsResponse struct {
	ID      string `json:"id"`
	OwnerID string `json:"owner_id"`
}

func chairPostChairs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := &chairPostChairsRequest{}
	if err := bindJSON(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Model == "" || req.ChairRegisterToken == "" {
		writeError(w, http.StatusBadRequest, errors.New("some of required fields(name, model, chair_register_token) are empty"))
		return
	}

	owner := &Owner{}
	if err := db.GetContext(ctx, owner, "SELECT * FROM owners WHERE chair_register_token = ?", req.ChairRegisterToken); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, errors.New("invalid chair_register_token"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	chairID := ulid.Make().String()
	accessToken := secureRandomStr(32)

	createdAt := time.Now().Truncate(time.Microsecond)
	_, err := db.ExecContext(
		ctx,
		"INSERT INTO chairs (id, owner_id, name, model, is_active, access_token, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		chairID, owner.ID, req.Name, req.Model, false, accessToken, createdAt, createdAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	stMu.Lock()
	chairStates[chairID] = &chairState{ID: chairID, OwnerID: owner.ID, Name: req.Name, Model: req.Model, Speed: modelSpeed[req.Model], CreatedAt: createdAt}
	stMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Path:  "/",
		Name:  "chair_session",
		Value: accessToken,
	})

	writeJSON(w, http.StatusCreated, &chairPostChairsResponse{
		ID:      chairID,
		OwnerID: owner.ID,
	})
}

type postChairActivityRequest struct {
	IsActive bool `json:"is_active"`
}

func chairPostActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	chair := ctx.Value("chair").(*Chair)

	req := &postChairActivityRequest{}
	if err := bindJSON(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err := db.ExecContext(ctx, "UPDATE chairs SET is_active = ? WHERE id = ?", req.IsActive, chair.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	stMu.Lock()
	if c := chairStates[chair.ID]; c != nil {
		c.Active = req.IsActive
	}
	stMu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

type chairPostCoordinateResponse struct {
	RecordedAt int64 `json:"recorded_at"`
}

func chairPostCoordinate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := &Coordinate{}
	if err := bindJSON(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	chair := ctx.Value("chair").(*Chair)
	now := time.Now().Truncate(time.Microsecond)

	stMu.Lock()
	c := chairStates[chair.ID]
	if c == nil {
		c = &chairState{ID: chair.ID, OwnerID: chair.OwnerID, Name: chair.Name, Model: chair.Model, Speed: modelSpeed[chair.Model], Active: chair.IsActive, CreatedAt: chair.CreatedAt}
		chairStates[chair.ID] = c
	}
	if c.HasLoc {
		c.TotalDist += abs(c.Lat-req.Latitude) + abs(c.Lon-req.Longitude)
	}
	c.HasLoc = true
	c.Lat, c.Lon, c.LocUpdatedAt = req.Latitude, req.Longitude, now
	newStatus := ""
	rideID := c.RideID
	if rideID != "" {
		if c.RideStatus == "ENROUTE" && req.Latitude == c.PickLat && req.Longitude == c.PickLon {
			newStatus = "PICKUP"
		} else if c.RideStatus == "CARRYING" && req.Latitude == c.DestLat && req.Longitude == c.DestLon {
			newStatus = "ARRIVED"
		}
		if newStatus != "" {
			c.RideStatus = newStatus
		}
	}
	stMu.Unlock()

	locBufMu.Lock()
	dirty[chair.ID] = struct{}{}
	locBufMu.Unlock()

	if newStatus != "" {
		enqueueStatus(rideID, newStatus)
		cacheStatus(rideID, newStatus)
		publishRide(ctx, rideID, newStatus, true, true)
	}

	writeJSON(w, http.StatusOK, &chairPostCoordinateResponse{
		RecordedAt: now.UnixMilli(),
	})
}

type simpleUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type chairGetNotificationResponse struct {
	Data         *chairGetNotificationResponseData `json:"data"`
	RetryAfterMs int                               `json:"retry_after_ms"`
}

type chairGetNotificationResponseData struct {
	RideID                string     `json:"ride_id"`
	User                  simpleUser `json:"user"`
	PickupCoordinate      Coordinate `json:"pickup_coordinate"`
	DestinationCoordinate Coordinate `json:"destination_coordinate"`
	Status                string     `json:"status"`
}

type postChairRidesRideIDStatusRequest struct {
	Status string `json:"status"`
}

func chairPostRideStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rideID := r.PathValue("ride_id")

	chair := ctx.Value("chair").(*Chair)

	req := &postChairRidesRideIDStatusRequest{}
	if err := bindJSON(r, req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ri, ok := getRideInfo(rideID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("ride not found"))
		return
	}
	if ri.Ride.ChairID.String != chair.ID {
		writeError(w, http.StatusBadRequest, errors.New("not assigned to this ride"))
		return
	}

	switch req.Status {
	// Acknowledge the ride
	case "ENROUTE":
	// After Picking up user
	case "CARRYING":
		if ri.Status != "PICKUP" {
			writeError(w, http.StatusBadRequest, errors.New("chair has not arrived yet"))
			return
		}
	default:
		writeError(w, http.StatusBadRequest, errors.New("invalid status"))
		return
	}

	enqueueStatus(rideID, req.Status)
	stMu.Lock()
	if c := chairStates[chair.ID]; c != nil && c.RideID == rideID {
		c.RideStatus = req.Status
	}
	stMu.Unlock()
	cacheStatus(rideID, req.Status)
	publishRide(ctx, rideID, req.Status, true, true)

	w.WriteHeader(http.StatusNoContent)
}
