package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type observedMetrics struct {
	started  int
	finished int
	status   int
}

func (m *observedMetrics) HTTPStarted(context.Context, string, string) { m.started++ }
func (m *observedMetrics) HTTPFinished(_ context.Context, _, _ string, status int, _ time.Duration) {
	m.finished++
	m.status = status
}

func TestPanicRecordedAs500InAllSignals(t *testing.T) {
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logOutput, nil))
	metrics := &observedMetrics{}

	previous := otel.GetTracerProvider()
	spanRecorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = provider.Shutdown(context.Background())
	})

	handler := Recover(logger)(Tracing()(Logging(logger)(Metrics(metrics)(http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) {
			panic("sensitive query details")
		},
	)))))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("HTTP status: got %d, want 500", response.Code)
	}
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "internal_error" {
		t.Fatalf("HTTP problem code: %q", problem.Code)
	}
	if metrics.started != 1 || metrics.finished != 1 || metrics.status != 500 {
		t.Fatalf("HTTP metric: %+v", metrics)
	}

	foundLog := false
	for _, line := range bytes.Split(bytes.TrimSpace(logOutput.Bytes()), []byte("\n")) {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatal(err)
		}
		if entry["msg"] == "request handled" {
			if entry["status"] != float64(500) || entry["level"] != "ERROR" {
				t.Fatalf("incorrect HTTP log: %v", entry)
			}
			foundLog = true
		}
	}
	if !foundLog {
		t.Fatal("request handled log missing")
	}

	foundSpan := false
	for _, span := range spanRecorder.Ended() {
		if span.Name() != "GET /health" {
			continue
		}
		attrs := map[string]attribute.Value{}
		for _, item := range span.Attributes() {
			attrs[string(item.Key)] = item.Value
		}
		if value, ok := attrs["http.response.status_code"]; !ok || value.AsInt64() != 500 {
			t.Fatalf("incorrect HTTP span status attributes: %v", attrs)
		}
		if span.Status().Code != codes.Error {
			t.Fatalf("HTTP span is not marked as error: %v", span.Status())
		}
		foundSpan = true
	}
	if !foundSpan {
		t.Fatal("HTTP span missing")
	}
}
