package create

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	domain "github.com/MrCat212/template/internal/domain/trip"
	api "github.com/MrCat212/template/internal/generated"
	"github.com/MrCat212/template/internal/http/domainerror"
	"github.com/MrCat212/template/internal/http/response"
	positioncreate "github.com/MrCat212/template/internal/usecase/trip/positions/create"
)

const maxBodyBytes = 1 << 20

type Handler struct{ usecase usecase }

func New(uc usecase) *Handler { return &Handler{usecase: uc} }

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, id api.TripId) {
	data, err := decodePosition(r.Body)
	if err != nil {
		domainerror.Write(w, r, &domain.ValidationError{Field: "body", Reason: err.Error()})
		return
	}

	saved, err := h.usecase.Execute(r.Context(), positioncreate.Input{
		TripID:     id,
		Latitude:   data.Coordinates.Latitude,
		Longitude:  data.Coordinates.Longitude,
		RecordedAt: data.RecordedAt,
	})
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, api.TripPosition{
		Id:          saved.ID,
		TripId:      saved.TripID,
		Coordinates: api.Coordinates{Latitude: saved.Latitude, Longitude: saved.Longitude},
		RecordedAt:  saved.RecordedAt,
	})
}

func decodePosition(reader io.Reader) (api.PositionData, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maxBodyBytes+1))
	if err != nil || len(body) > maxBodyBytes {
		return api.PositionData{}, errors.New("invalid or too large body")
	}
	if !hasRequiredFields(body) {
		return api.PositionData{}, errors.New("missing required fields or invalid JSON")
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var data api.PositionData
	if err := dec.Decode(&data); err != nil {
		return api.PositionData{}, errors.New("invalid JSON body")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return api.PositionData{}, errors.New("expected exactly one JSON object")
	}
	return data, nil
}

func hasRequiredFields(body []byte) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return false
	}
	if !present(fields, "coordinates") || !present(fields, "recorded_at") {
		return false
	}

	var coords map[string]json.RawMessage
	if err := json.Unmarshal(fields["coordinates"], &coords); err != nil {
		return false
	}
	return present(coords, "latitude") && present(coords, "longitude")
}

func present(fields map[string]json.RawMessage, key string) bool {
	raw, exists := fields[key]
	return exists && !bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
