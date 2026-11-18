package positions

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestQueuedCancellationMetricLabel(t *testing.T) {
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
	metrics.recordQueuedCancelled(ctx)

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &collected); err != nil {
		t.Fatal(err)
	}

	for _, scope := range collected.ScopeMetrics {
		for _, instrument := range scope.Metrics {
			if instrument.Name != "tripgo.position_requests" {
				continue
			}
			sum, ok := instrument.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("expected counter, got %T", instrument.Data)
			}
			for _, point := range sum.DataPoints {
				for _, attr := range point.Attributes.ToSlice() {
					if attr.Key == "result" && attr.Value.AsString() == "cancelled" && point.Value == 1 {
						return
					}
				}
			}
		}
	}
	t.Fatal("missing tripgo.position_requests{result=cancelled} with value 1")
}
