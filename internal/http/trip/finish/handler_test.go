package finish_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domain "github.com/MrCat212/template/internal/domain/trip"
	api "github.com/MrCat212/template/internal/generated"
	handler "github.com/MrCat212/template/internal/http/trip/finish"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestFinishSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockusecase(ctrl)
	id := uuid.New()
	at := time.Now().UTC()
	uc.EXPECT().Execute(gomock.Any(), id).Return(domain.Trip{
		ID:         id,
		Status:     domain.StatusCompleted,
		StartedAt:  at.Add(-time.Minute),
		FinishedAt: &at,
	}, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id.String()+"/finish", nil)
	handler.New(uc).Handle(rec, req, id)

	if rec.Code != http.StatusOK {
		t.Fatalf("Ожидали 200, получили %d: %s", rec.Code, rec.Body.String())
	}
	var result api.Trip
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Id != id || result.Status != api.TripStatus(domain.StatusCompleted) || result.FinishedAt == nil {
		t.Errorf("Неправильный ответ: %+v", result)
	}
}

func TestFinishErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not found", domain.ErrTripNotFound, http.StatusNotFound, "trip_not_found"},
		{"completed", domain.ErrTripCompleted, http.StatusConflict, "trip_completed"},
		{"internal error", errors.New("secret database password"), http.StatusInternalServerError, "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			uc := NewMockusecase(ctrl)
			id := uuid.New()
			uc.EXPECT().Execute(gomock.Any(), id).Return(domain.Trip{}, tt.err)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id.String()+"/finish", nil)
			handler.New(uc).Handle(rec, req, id)

			if rec.Code != tt.status {
				t.Fatalf("Ожидали %d, получили %d: %s", tt.status, rec.Code, rec.Body.String())
			}
			var result api.Problem
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Code != tt.code {
				t.Errorf("Ожидали код %q, получили %q", tt.code, result.Code)
			}
			if strings.Contains(rec.Body.String(), "secret database password") {
				t.Error("Внутренняя ошибка попала в HTTP-ответ")
			}
		})
	}
}
