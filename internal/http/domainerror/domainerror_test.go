package domainerror

import (
	"errors"
	"net/http"
	"testing"

	domain "github.com/MrCat212/template/internal/domain/trip"
)

func TestProblem(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"invalid request", &domain.ValidationError{Field: "price", Reason: "negative"}, http.StatusBadRequest, "invalid_request"},
		{"driver busy", domain.ErrDriverBusy, http.StatusConflict, "driver_busy"},
		{"trip not found", domain.ErrTripNotFound, http.StatusNotFound, "trip_not_found"},
		{"trip completed", domain.ErrTripCompleted, http.StatusConflict, "trip_completed"},
		{"internal error", errors.New("database password secret"), http.StatusInternalServerError, "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, p := Problem(tt.err)
			if status != tt.status || p.Code != tt.code {
				t.Errorf("status %d, code %q; expected %d, %q", status, p.Code, tt.status, tt.code)
			}
			if tt.code == "internal_error" && p.Detail != nil && *p.Detail == tt.err.Error() {
				t.Fatal("internal error details leaked to client")
			}
		})
	}
}
