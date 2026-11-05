package create

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"

	domain "github.com/MrCat212/template/internal/domain/trip"
	api "github.com/MrCat212/template/internal/generated"
	"github.com/MrCat212/template/internal/http/domainerror"
	"github.com/MrCat212/template/internal/http/response"
	"github.com/MrCat212/template/internal/http/trip/convert"
	usecasecreate "github.com/MrCat212/template/internal/usecase/trip/create"
)

type Handler struct {
	usecase usecase
}

func New(uc usecase) *Handler {
	return &Handler{usecase: uc}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		invalid(w, r, "body", "invalid or too large")
		return
	}

	if !hasRequiredFields(body) {
		invalid(w, r, "body", "missing required fields")
		return
	}

	var data api.TripData
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&data); err != nil {
		invalid(w, r, "body", "invalid JSON")
		return
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		invalid(w, r, "body", "expected one JSON object")
		return
	}

	if data.UserId == uuid.Nil {
		invalid(w, r, "user_id", "must not be empty")
		return
	}
	if data.DriverId == uuid.Nil {
		invalid(w, r, "driver_id", "must not be empty")
		return
	}
	if data.Price < 0 {
		invalid(w, r, "price", "must be nonnegative")
		return
	}
	if data.StartPoint.Latitude < -90 || data.StartPoint.Latitude > 90 ||
		data.StartPoint.Longitude < -180 || data.StartPoint.Longitude > 180 {
		invalid(w, r, "start_point", "invalid coordinates")
		return
	}
	if data.EndPoint.Latitude < -90 || data.EndPoint.Latitude > 90 ||
		data.EndPoint.Longitude < -180 || data.EndPoint.Longitude > 180 {
		invalid(w, r, "end_point", "invalid coordinates")
		return
	}

	created, err := h.usecase.Execute(r.Context(), usecasecreate.Input{
		UserID: data.UserId, DriverID: data.DriverId,
		StartLatitude: data.StartPoint.Latitude, StartLongitude: data.StartPoint.Longitude,
		EndLatitude: data.EndPoint.Latitude, EndLongitude: data.EndPoint.Longitude,
		Price: data.Price,
	})
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+created.ID.String())
	response.JSON(w, http.StatusCreated, convert.ToAPITrip(created))
}

func invalid(w http.ResponseWriter, r *http.Request, field, reason string) {
	domainerror.Write(w, r, &domain.ValidationError{Field: field, Reason: reason})
}

func hasRequiredFields(body []byte) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return false
	}

	for _, name := range []string{"user_id", "driver_id", "start_point", "end_point", "price"} {
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
		for _, key := range []string{"latitude", "longitude"} {
			value, ok := coords[key]
			if !ok || string(value) == "null" {
				return false
			}
		}
	}
	return true
}
