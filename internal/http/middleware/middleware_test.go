package middleware

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/MrCat212/template/internal/generated"
)

func TestRoute(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/health", "/health"},
		{"/ready", "/ready"},
		{"/api/v1/trips", "/api/v1/trips"},
		{"/api/v1/trips/123", "/api/v1/trips/{tripId}"},
		{"/api/v1/trips/123/finish", "/api/v1/trips/{tripId}/finish"},
		{"/something-else", "unmatched"},
	}
	for _, test := range cases {
		t.Run(test.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			if got := Route(req); got != test.want {
				t.Errorf("Route(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestRecover(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := Recover(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("private database credentials")
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	var problem api.Problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "internal_error" {
		t.Errorf("code = %q, want internal_error", problem.Code)
	}
	if problem.Detail != nil && *problem.Detail == "private database credentials" {
		t.Fatal("panic details leaked")
	}
}
