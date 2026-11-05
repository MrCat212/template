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
}

func New(uc usecase) *Handler {
	return &Handler{usecase: uc}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, id api.TripId) {
	finished, err := h.usecase.Execute(r.Context(), id)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}
	response.JSON(w, http.StatusOK, convert.ToAPITrip(finished))
}
