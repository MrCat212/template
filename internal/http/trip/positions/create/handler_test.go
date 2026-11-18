package create_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domain "github.com/MrCat212/template/internal/domain/trip"
	api "github.com/MrCat212/template/internal/generated"
	"github.com/MrCat212/template/internal/http/trip/positions/create"
	positioncreate "github.com/MrCat212/template/internal/usecase/trip/positions/create"
)

func TestCreatePositionHTTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockusecase(ctrl)
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	body := fmt.Sprintf(`{"coordinates":{"latitude":55.75,"longitude":37.61},"recorded_at":%q}`, now.Format(time.RFC3339Nano))

	uc.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ctx context.Context, input positioncreate.Input) (domain.Position, error) {
			if input.TripID != id || input.Latitude != 55.75 || input.Longitude != 37.61 || !input.RecordedAt.Equal(now) {
				t.Errorf("unexpected input: %+v", input)
			}
			return domain.Position{ID: 42, TripID: id, Latitude: input.Latitude, Longitude: input.Longitude, RecordedAt: input.RecordedAt}, nil
		},
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id.String()+"/positions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	create.New(uc).Handle(rec, req, id)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, expected 201, body=%s", rec.Code, rec.Body.String())
	}
	var response api.TripPosition
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Id != 42 || response.TripId != id {
		t.Errorf("unexpected result: %+v", response)
	}
}

func TestCreatePositionInvalidBody(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockusecase(ctrl)
	id := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id.String()+"/positions", strings.NewReader(`{"coordinates":{"latitude":55.75},"recorded_at":"2026-11-15T00:00:00Z"}`))
	rec := httptest.NewRecorder()
	create.New(uc).Handle(rec, req, id)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, expected 400", rec.Code)
	}
	var problem api.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "invalid_request" {
		t.Errorf("code = %q", problem.Code)
	}
}

func TestCreatePositionCompleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockusecase(ctrl)
	id := uuid.New()
	uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(domain.Position{}, domain.ErrTripCompleted)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id.String()+"/positions", strings.NewReader(`{"coordinates":{"latitude":55.75,"longitude":37.61},"recorded_at":"2026-11-15T00:00:00Z"}`))
	rec := httptest.NewRecorder()
	create.New(uc).Handle(rec, req, id)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, expected 409", rec.Code)
	}
}
