package postgres

import (
	"context"
	"testing"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestQueryTracerAttributes(t *testing.T) {
	const insertOperation = "INSERT"

	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithView(sdkmetric.NewView(
			sdkmetric.Instrument{
				Name:  "db.client.operation.duration",
				Scope: instrumentation.Scope{Name: "github.com/exaring/otelpgx"},
			},
			sdkmetric.Stream{Aggregation: sdkmetric.AggregationDrop{}},
		)),
	)
	defer func() { _ = meterProvider.Shutdown(context.Background()) }()

	spans := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	defer func() { _ = tracerProvider.Shutdown(context.Background()) }()

	duration, err := meterProvider.Meter("trip-service.postgres").Float64Histogram(
		"db.client.operation.duration", metric.WithUnit("s"),
	)
	if err != nil {
		t.Fatal(err)
	}
	tracer := &queryTracer{
		Tracer: otelpgx.NewTracer(
			otelpgx.WithTracerProvider(tracerProvider),
			otelpgx.WithMeterProvider(meterProvider),
			otelpgx.WithDisableSQLStatementInAttributes(),
			otelpgx.WithMetricOperationName(otelpgx.SQLOperationName),
		),
		duration: duration,
	}

	ctx, parent := tracerProvider.Tracer("test").Start(context.Background(), "parent")

	failed := tracer.TraceQueryStart(WithCollection(ctx, "trips"), nil, pgx.TraceQueryStartData{
		SQL: "INSERT INTO trips (id) VALUES ($1)",
	})
	tracer.TraceQueryEnd(failed, nil, pgx.TraceQueryEndData{Err: &pgconn.PgError{Code: "23505"}})

	success := tracer.TraceQueryStart(WithCollection(ctx, "trip_status_history"), nil, pgx.TraceQueryStartData{
		SQL: "SELECT id FROM trip_status_history",
	})
	tracer.TraceQueryEnd(success, nil, pgx.TraceQueryEndData{})
	parent.End()

	foundFailedSpan := false
	for _, span := range spans.Ended() {
		if span.Name() != insertOperation {
			continue
		}
		attrs := attributes(span.Attributes())
		if attrs["db.collection.name"] != "trips" || attrs["error.type"] != "23505" ||
			attrs["db.operation.name"] != insertOperation {
			t.Fatalf("wrong database span attributes: %v", attrs)
		}
		foundFailedSpan = true
	}
	if !foundFailedSpan {
		t.Fatal("INSERT database span not found")
	}

	var output metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	foundFailedMetric := false
	foundSuccessMetric := false
	for _, scope := range output.ScopeMetrics {
		for _, instrument := range scope.Metrics {
			if instrument.Name != "db.client.operation.duration" {
				continue
			}
			histogram, ok := instrument.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("unexpected duration aggregation type: %T", instrument.Data)
			}
			for _, point := range histogram.DataPoints {
				attrs := attributes(point.Attributes.ToSlice())
				switch attrs["db.collection.name"] {
				case "trips":
					if attrs["db.operation.name"] != insertOperation || attrs["error.type"] != "23505" {
						t.Fatalf("failed query metric attributes: %v", attrs)
					}
					foundFailedMetric = true
				case "trip_status_history":
					if attrs["db.operation.name"] != "SELECT" {
						t.Fatalf("successful query metric attributes: %v", attrs)
					}
					if _, exists := attrs["error.type"]; exists {
						t.Fatalf("successful query must not have error.type: %v", attrs)
					}
					foundSuccessMetric = true
				}
			}
		}
	}
	if !foundFailedMetric || !foundSuccessMetric {
		t.Fatalf("missing database metrics: failed=%v, success=%v", foundFailedMetric, foundSuccessMetric)
	}
}

func attributes(attrs []attribute.KeyValue) map[string]string {
	result := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		result[string(attr.Key)] = attr.Value.AsString()
	}
	return result
}
