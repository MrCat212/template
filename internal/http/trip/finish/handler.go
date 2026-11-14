package finish

import (
	"net/http"

	api "github.com/MrCat212/template/internal/generated"
	"github.com/MrCat212/template/internal/http/domainerror"
	"github.com/MrCat212/template/internal/http/response"
	"github.com/MrCat212/template/internal/http/trip/convert"
)

type Handler struct {
	usecase usecase
	events  eventRecorder
}

func New(uc usecase, events ...eventRecorder) *Handler {
	h := &Handler{usecase: uc}
	if len(events) > 0 {
		h.events = events[0]
	}
	return h
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, id api.TripId) {
	finished, err := h.usecase.Execute(r.Context(), id)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}
	if h.events != nil {
		h.events.Completed(r.Context(), finished.ID)
	}

	response.JSON(w, http.StatusOK, convert.ToAPITrip(finished))
}
