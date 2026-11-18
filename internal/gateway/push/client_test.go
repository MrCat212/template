package push_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MrCat212/template/internal/domain/trip"
	"github.com/MrCat212/template/internal/gateway/push"
)

func TestRequestPositionSuccess(t *testing.T) {
	item := trip.StaleTrip{
		ID:       uuid.New(),
		DriverID: uuid.New(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		if r.URL.Path != "/api/v1/push" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected JSON content type")
		}

		var body struct {
			RequestID   string            `json:"request_id"`
			RecipientID uuid.UUID         `json:"recipient_id"`
			Kind        string            `json:"kind"`
			Data        map[string]string `json:"data"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}

		if body.RequestID == "" {
			t.Error("request_id must not be empty")
		}

		if body.RecipientID != item.DriverID {
			t.Error("wrong recipient_id")
		}

		if body.Kind != "REQUEST_POSITION" {
			t.Error("wrong push kind")
		}

		if body.Data["trip_id"] != item.ID.String() {
			t.Error("wrong trip_id")
		}

		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client, err := push.New(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.RequestPosition(context.Background(), item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestPositionHTTPError(t *testing.T) {
	for _, status := range []int{
		http.StatusBadRequest,
		http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				w.WriteHeader(status)
			}))
			defer server.Close()

			client, err := push.New(server.URL, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()

			err = client.RequestPosition(
				context.Background(),
				trip.StaleTrip{
					ID:       uuid.New(),
					DriverID: uuid.New(),
				},
			)

			var statusErr *push.HTTPStatusError
			if !errors.As(err, &statusErr) {
				t.Fatalf("expected HTTPStatusError, got %v", err)
			}

			if statusErr.StatusCode != status {
				t.Errorf(
					"expected HTTP %d, got %d",
					status,
					statusErr.StatusCode,
				)
			}
		})
	}
}

func TestRequestPositionCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(
		w http.ResponseWriter,
		_ *http.Request,
	) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client, err := push.New(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = client.RequestPosition(ctx, trip.StaleTrip{
		ID:       uuid.New(),
		DriverID: uuid.New(),
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
