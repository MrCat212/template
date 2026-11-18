package positions

import (
	"context"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestPositionMetricInstruments(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() {
		if err := provider.Shutdown(ctx); err != nil {
			t.Errorf("shutdown metric provider: %v", err)
		}
	}()

	metrics, err := newPoolMetricsWithMeter(provider.Meter("trip-service.worker.positions"))
	if err != nil {
		t.Fatal(err)
	}

	metrics.recordEnqueue(ctx, resultQueued)
	metrics.recordEnqueue(ctx, resultDropped)
	metrics.queueSize.Add(ctx, 1)
	metrics.workersActive.Add(ctx, 1)
	metrics.recordRequest(ctx, "success", time.Millisecond)

	var result metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &result); err != nil {
		t.Fatal(err)
	}

	names := make(map[string]bool)
	for _, scope := range result.ScopeMetrics {
		for _, item := range scope.Metrics {
			names[item.Name] = true
		}
	}
	for _, name := range []string{
		"tripgo.stale_checks",
		"tripgo.position_requests.queue.size",
		"tripgo.position_requests.workers.active",
		"tripgo.position_requests",
		"tripgo.position_requests.duration",
	} {
		if !names[name] {
			t.Errorf("missing metric: %s", name)
		}
	}
}
