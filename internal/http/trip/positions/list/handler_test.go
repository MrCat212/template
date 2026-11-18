package list_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domain "github.com/MrCat212/template/internal/domain/trip"
	api "github.com/MrCat212/template/internal/generated"
	"github.com/MrCat212/template/internal/http/trip/positions/list"
)

func TestListPositionsHTTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockusecase(ctrl)
	id := uuid.New()
	uc.EXPECT().Execute(gomock.Any(), id).Return([]domain.Position{{ID: 11, TripID: id, Latitude: 55.75, Longitude: 37.61, RecordedAt: time.Now().UTC()}}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+id.String()+"/positions", nil)
	rec := httptest.NewRecorder()
	list.New(uc).Handle(rec, req, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, expected 200", rec.Code)
	}
	var data api.TripPositionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Positions) != 1 || data.Positions[0].Id != 11 {
		t.Errorf("unexpected positions: %+v", data.Positions)
	}
}

func TestListPositionsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockusecase(ctrl)
	id := uuid.New()
	uc.EXPECT().Execute(gomock.Any(), id).Return(nil, domain.ErrTripNotFound)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+id.String()+"/positions", nil)
	rec := httptest.NewRecorder()
	list.New(uc).Handle(rec, req, id)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, expected 404", rec.Code)
	}
}
