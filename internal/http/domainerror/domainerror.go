package domainerror

import (
	"errors"
	"net/http"
	"strings"

	domain "github.com/MrCat212/template/internal/domain/trip"
	api "github.com/MrCat212/template/internal/generated"
	"github.com/MrCat212/template/internal/http/response"
)

func Problem(err error) (int, api.Problem) {
	status := http.StatusInternalServerError
	code := "internal_error"
	title := "Internal error"
	detail := "Internal server error"

	var validationErr *domain.ValidationError
	switch {
	case errors.As(err, &validationErr):
		status = http.StatusBadRequest
		code = "invalid_request"
		title = "Invalid request"
		detail = validationErr.Error()
	case errors.Is(err, domain.ErrDriverBusy):
		status = http.StatusConflict
		code = "driver_busy"
		title = "Driver busy"
		detail = "Driver already has an active trip"
	case errors.Is(err, domain.ErrTripNotFound):
		status = http.StatusNotFound
		code = "trip_not_found"
		title = "Trip not found"
		detail = "Trip does not exist"
	case errors.Is(err, domain.ErrTripCompleted):
		status = http.StatusConflict
		code = "trip_completed"
		title = "Trip completed"
		detail = "Trip is already completed"
	}

	return status, api.Problem{
		Type:   "https://tripgo.example/problems/" + strings.ReplaceAll(code, "_", "-"),
		Title:  title,
		Status: int32(status),
		Code:   code,
		Detail: &detail,
	}
}

func Write(w http.ResponseWriter, r *http.Request, err error) {
	status, body := Problem(err)
	instance := r.URL.Path
	body.Instance = &instance
	response.Problem(w, status, body)
}

func HandleAPIError(w http.ResponseWriter, r *http.Request, err error) {
	Write(w, r, &domain.ValidationError{Field: "request", Reason: "invalid URL parameter"})
}
