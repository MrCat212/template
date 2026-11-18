package positions

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type poolMetrics struct {
	staleChecks     metric.Int64Counter
	queueSize       metric.Int64UpDownCounter
	workersActive   metric.Int64UpDownCounter
	requests        metric.Int64Counter
	requestDuration metric.Float64Histogram
}

func newPoolMetrics() (*poolMetrics, error) {
	return newPoolMetricsWithMeter(otel.Meter("trip-service.worker.positions"))
}

func newPoolMetricsWithMeter(meter metric.Meter) (*poolMetrics, error) {
	m := &poolMetrics{}
	var err error

	m.staleChecks, err = meter.Int64Counter("tripgo.stale_checks", metric.WithUnit("{check}"))
	if err != nil {
		return nil, fmt.Errorf("create stale check counter: %w", err)
	}
	m.queueSize, err = meter.Int64UpDownCounter("tripgo.position_requests.queue.size", metric.WithUnit("{task}"))
	if err != nil {
		return nil, fmt.Errorf("create positions queue gauge: %w", err)
	}
	m.workersActive, err = meter.Int64UpDownCounter("tripgo.position_requests.workers.active", metric.WithUnit("{worker}"))
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}
	m.requests, err = meter.Int64Counter("tripgo.position_requests", metric.WithUnit("{request}"))
	if err != nil {
		return nil, fmt.Errorf("create position requests counter: %w", err)
	}
	m.requestDuration, err = meter.Float64Histogram("tripgo.position_requests.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, fmt.Errorf("create position request duration histogram: %w", err)
	}

	return m, nil
}

func (m *poolMetrics) recordEnqueue(ctx context.Context, result string) {
	m.staleChecks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
	if result == resultDropped {
		m.requests.Add(ctx, 1, metric.WithAttributes(attribute.String("result", resultDropped)))
	}
}

func (m *poolMetrics) recordRequest(ctx context.Context, result string, elapsed time.Duration) {
	opts := metric.WithAttributes(attribute.String("result", result))
	m.requests.Add(ctx, 1, opts)
	m.requestDuration.Record(ctx, elapsed.Seconds(), opts)
}

// recordQueuedCancelled counts a position that arrived before its queued job started.
// No Push Service request was made, so no request-duration observation is recorded.
func (m *poolMetrics) recordQueuedCancelled(ctx context.Context) {
	m.requests.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "cancelled")))
}
