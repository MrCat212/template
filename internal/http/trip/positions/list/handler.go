package list

import (
	"net/http"

	api "github.com/MrCat212/template/internal/generated"
	"github.com/MrCat212/template/internal/http/domainerror"
	"github.com/MrCat212/template/internal/http/response"
)

type Handler struct{ usecase usecase }

func New(uc usecase) *Handler { return &Handler{usecase: uc} }

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, id api.TripId) {
	positions, err := h.usecase.Execute(r.Context(), id)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	items := make([]api.TripPosition, 0, len(positions))
	for _, p := range positions {
		items = append(items, api.TripPosition{
			Id:          p.ID,
			TripId:      p.TripID,
			Coordinates: api.Coordinates{Latitude: p.Latitude, Longitude: p.Longitude},
			RecordedAt:  p.RecordedAt,
		})
	}
	response.JSON(w, http.StatusOK, api.TripPositionsResponse{Positions: items})
}
