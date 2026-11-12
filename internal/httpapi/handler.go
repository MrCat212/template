package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	api "github.com/MrCat212/template/internal/generated"
	"github.com/MrCat212/template/internal/trip"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	service      *trip.Service
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewHandler(
	service *trip.Service,
	pool *pgxpool.Pool,
	queryTimeout time.Duration,
) *Handler {
	return &Handler{
		service:      service,
		pool:         pool,
		queryTimeout: queryTimeout,
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.HealthResponse{
		Status: api.HealthResponseStatus("ok"),
	})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	status := api.HealthResponseStatus("ok")
	httpStatus := http.StatusOK

	if err := h.pool.Ping(ctx); err != nil {
		status = api.HealthResponseStatus("unavailable")
		httpStatus = http.StatusServiceUnavailable
	}

	writeJSON(w, httpStatus, api.HealthResponse{
		Status: status,
	})
}

func toAPITrip(t trip.Trip) api.Trip {
	return api.Trip{
		Id:       t.ID,
		UserId:   t.UserID,
		DriverId: t.DriverID,

		StartPoint: api.Coordinates{
			Latitude:  t.StartLatitude,
			Longitude: t.StartLongitude,
		},

		EndPoint: api.Coordinates{
			Latitude:  t.EndLatitude,
			Longitude: t.EndLongitude,
		},

		Price:      t.Price,
		Status:     api.TripStatus(t.Status),
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,

		LastPositionAt: nil,
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}

func writeProblem(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	code string,
	title string,
	detail string,
) {
	problem := api.Problem{
		Type: "https://tripgo.example/problems/" +
			strings.ReplaceAll(code, "_", "-"),
		Title:    title,
		Status:   int32(status),
		Code:     code,
		Detail:   &detail,
		Instance: &r.URL.Path,
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(problem)
}

func (h *Handler) CreateTrip(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateTripParams,
) {
	body, err := io.ReadAll(r.Body)
	if err != nil || !hasRequiredFields(body) {
		writeProblem(w, r, 400, "invalid_request",
			"Invalid request", "Missing required fields")
		return
	}

	var data api.TripData

	decoder := json.NewDecoder(bytes.NewReader(body))

	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&data); err != nil {
		writeProblem(w, r, 400, "invalid_request",
			"Invalid request", "Invalid JSON body")
		return
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeProblem(w, r, 400, "invalid_request",
			"Invalid request", "Request must contain one JSON object")
		return
	}

	if data.UserId == uuid.Nil || data.DriverId == uuid.Nil ||
		data.Price < 0 ||
		data.StartPoint.Latitude < -90 || data.StartPoint.Latitude > 90 ||
		data.StartPoint.Longitude < -180 || data.StartPoint.Longitude > 180 ||
		data.EndPoint.Latitude < -90 || data.EndPoint.Latitude > 90 ||
		data.EndPoint.Longitude < -180 || data.EndPoint.Longitude > 180 {

		writeProblem(w, r, 400, "invalid_request",
			"Invalid request", "Invalid trip data")
		return
	}

	input := trip.CreateInput{
		UserID:         data.UserId,
		DriverID:       data.DriverId,
		StartLatitude:  data.StartPoint.Latitude,
		StartLongitude: data.StartPoint.Longitude,
		EndLatitude:    data.EndPoint.Latitude,
		EndLongitude:   data.EndPoint.Longitude,
		Price:          data.Price,
	}

	createdTrip, err := h.service.Create(r.Context(), input)

	if errors.Is(err, trip.ErrDriverBusy) {
		writeProblem(w, r, 409, "driver_busy",
			"Driver busy", "Driver already has an active trip")
		return
	}

	if err != nil {
		writeProblem(w, r, 500, "internal_error",
			"Internal error", "Failed to create trip")
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+createdTrip.ID.String())

	writeJSON(w, http.StatusCreated, toAPITrip(createdTrip))
}
func (h *Handler) GetTrip(
	w http.ResponseWriter,
	r *http.Request,
	tripId api.TripId,
) {
	foundTrip, err := h.service.GetByID(r.Context(), tripId)

	if errors.Is(err, trip.ErrTripNotFound) {
		writeProblem(w, r, 404, "trip_not_found",
			"Trip not found", "Trip does not exist")
		return
	}

	if err != nil {
		writeProblem(w, r, 500, "internal_error",
			"Internal error", "Failed to get trip")
		return
	}

	writeJSON(w, http.StatusOK, toAPITrip(foundTrip))
}
func (h *Handler) FinishTrip(
	w http.ResponseWriter,
	r *http.Request,
	tripId api.TripId,
) {
	finishedTrip, err := h.service.Finish(r.Context(), tripId)

	if errors.Is(err, trip.ErrTripNotFound) {
		writeProblem(w, r, 404, "trip_not_found",
			"Trip not found", "Trip does not exist")
		return
	}

	if errors.Is(err, trip.ErrTripCompleted) {
		writeProblem(w, r, 409, "trip_completed",
			"Trip completed", "Trip is already completed")
		return
	}

	if err != nil {
		writeProblem(w, r, 500, "internal_error",
			"Internal error", "Failed to finish trip")
		return
	}

	writeJSON(w, http.StatusOK, toAPITrip(finishedTrip))
}
func hasRequiredFields(body []byte) bool {
	var fields map[string]json.RawMessage

	if err := json.Unmarshal(body, &fields); err != nil {
		return false
	}

	required := []string{
		"user_id",
		"driver_id",
		"start_point",
		"end_point",
		"price",
	}

	for _, name := range required {
		value, ok := fields[name]
		if !ok || string(value) == "null" {
			return false
		}
	}

	for _, name := range []string{"start_point", "end_point"} {
		var coords map[string]json.RawMessage

		if err := json.Unmarshal(fields[name], &coords); err != nil {
			return false
		}

		for _, field := range []string{"latitude", "longitude"} {
			value, ok := coords[field]
			if !ok || string(value) == "null" {
				return false
			}
		}
	}

	return true
}
func HandleAPIError(w http.ResponseWriter, r *http.Request, err error) {
	writeProblem(w, r, http.StatusBadRequest,
		"invalid_request", "Invalid request", "Invalid URL parameter")
}