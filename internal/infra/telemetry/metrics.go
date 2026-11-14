package telemetry

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func (t *Telemetry) HTTPStarted(ctx context.Context, method, route string) {
	t.active.Add(ctx, 1, metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("http.route", route),
	))
}

func (t *Telemetry) HTTPFinished(ctx context.Context, method, route string, status int, elapsed time.Duration) {
	t.active.Add(ctx, -1, metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("http.route", route),
	))
	t.duration.Record(ctx, elapsed.Seconds(), metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("http.route", route),
		attribute.Int("http.response.status_code", status),
	))
}

func (t *Telemetry) Created(ctx context.Context, id uuid.UUID) {
	t.created.Add(ctx, 1)
	t.Logger.InfoContext(ctx, "trip created", "trip_id", id.String())
}

func (t *Telemetry) Completed(ctx context.Context, id uuid.UUID) {
	t.completed.Add(ctx, 1)
	t.Logger.InfoContext(ctx, "trip completed", "trip_id", id.String())
}
