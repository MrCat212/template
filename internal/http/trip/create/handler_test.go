package create_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domain "github.com/MrCat212/template/internal/domain/trip"
	api "github.com/MrCat212/template/internal/generated"
	handler "github.com/MrCat212/template/internal/http/trip/create"
	createuc "github.com/MrCat212/template/internal/usecase/trip/create"
)

func validBody(t *testing.T) string {
	t.Helper()

	body, err := json.Marshal(api.TripData{
		UserId:     uuid.New(),
		DriverId:   uuid.New(),
		StartPoint: api.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   api.Coordinates{Latitude: 55.76, Longitude: 37.62},
		Price:      500,
	})
	if err != nil {
		t.Fatal(err)
	}

	return string(body)
}

func TestCreateSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockusecase(ctrl)
	id := uuid.New()

	uc.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, input createuc.Input) (domain.Trip, error) {
			if input.Price != 500 || input.StartLatitude != 55.75 || input.EndLongitude != 37.62 {
				t.Errorf("Неправильные входные данные: %+v", input)
			}
			return domain.Trip{
				ID:        id,
				UserID:    input.UserID,
				DriverID:  input.DriverID,
				Status:    domain.StatusActive,
				Price:     input.Price,
				StartedAt: time.Now().UTC(),
			}, nil
		},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(validBody(t)))
	handler.New(uc).Handle(rec, req, api.CreateTripParams{})

	if rec.Code != http.StatusCreated {
		t.Fatalf("Ожидали 201, получили %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "/api/v1/trips/"+id.String() {
		t.Errorf("Неправильный Location: %s", rec.Header().Get("Location"))
	}

	var result api.Trip
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Id != id || result.Status != api.TripStatus(domain.StatusActive) {
		t.Errorf("Неправильный ответ: %+v", result)
	}
}

func TestCreateErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"driver busy", domain.ErrDriverBusy, http.StatusConflict, "driver_busy"},
		{"internal error", errors.New("secret database password"), http.StatusInternalServerError, "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			uc := NewMockusecase(ctrl)
			uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(domain.Trip{}, tt.err)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(validBody(t)))
			handler.New(uc).Handle(rec, req, api.CreateTripParams{})

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

func TestCreateInvalidBody(t *testing.T) {
	badPrice := validBody(t)
	var fields map[string]any
	if err := json.Unmarshal([]byte(badPrice), &fields); err != nil {
		t.Fatal(err)
	}
	fields["price"] = -500
	badPriceBytes, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		body string
	}{
		{"missing fields", `{}`},
		{"malformed JSON", `{bad`},
		{"negative price", string(badPriceBytes)},
		{"two objects", validBody(t) + validBody(t)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			uc := NewMockusecase(ctrl)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(tt.body))
			handler.New(uc).Handle(rec, req, api.CreateTripParams{})

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("Ожидали 400, получили %d: %s", rec.Code, rec.Body.String())
			}
			var result api.Problem
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Code != "invalid_request" {
				t.Errorf("Ожидали invalid_request, получили %s", result.Code)
			}
		})
	}
}
