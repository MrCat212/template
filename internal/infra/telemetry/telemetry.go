package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Telemetry struct {
	Logger    *slog.Logger
	tracer    *sdktrace.TracerProvider
	meter     *sdkmetric.MeterProvider
	duration  metric.Float64Histogram
	active    metric.Int64UpDownCounter
	created   metric.Int64Counter
	completed metric.Int64Counter
}

func New(ctx context.Context, rawLevel string) (*Telemetry, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToUpper(rawLevel))); err != nil {
		return nil, fmt.Errorf("parse LOG_LEVEL: %w", err)
	}

	logger := slog.New(&traceHandler{
		next: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
	}).With("service", "trip-service")

	if protocol := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"); protocol != "" && protocol != "http/protobuf" {
		return nil, fmt.Errorf("unsupported OTEL_EXPORTER_OTLP_PROTOCOL %q: expected http/protobuf", protocol)
	}

	sampler, err := samplerFromEnv()
	if err != nil {
		return nil, err
	}

	res := resource.NewWithAttributes("",
		attribute.String("service.name", envOr("OTEL_SERVICE_NAME", "trip-service")),
		attribute.String("service.version", envOr("SERVICE_VERSION", "dev")),
		attribute.String("service.namespace", "tripgo"),
		attribute.String("deployment.environment", envOr("DEPLOYMENT_ENVIRONMENT", "local")),
	)

	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}

	tracer := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter),
	)

	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		_ = tracer.Shutdown(ctx)
		return nil, fmt.Errorf("create metric exporter: %w", err)
	}

	meter := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(5*time.Second))),
		sdkmetric.WithView(sdkmetric.NewView(
			sdkmetric.Instrument{
				Name:  "db.client.operation.duration",
				Scope: instrumentation.Scope{Name: "github.com/exaring/otelpgx"},
			},
			sdkmetric.Stream{Aggregation: sdkmetric.AggregationDrop{}},
		)),
	)

	t := &Telemetry{Logger: logger, tracer: tracer, meter: meter}
	instruments := meter.Meter("trip-service")

	t.duration, err = instruments.Float64Histogram("http.server.request.duration", metric.WithUnit("s"))
	if err != nil {
		_ = t.Shutdown(ctx)
		return nil, fmt.Errorf("create request duration metric: %w", err)
	}
	t.active, err = instruments.Int64UpDownCounter("http.server.active_requests", metric.WithUnit("{request}"))
	if err != nil {
		_ = t.Shutdown(ctx)
		return nil, fmt.Errorf("create active requests metric: %w", err)
	}
	t.created, err = instruments.Int64Counter("tripgo.trips.created", metric.WithUnit("{trip}"))
	if err != nil {
		_ = t.Shutdown(ctx)
		return nil, fmt.Errorf("create trip created metric: %w", err)
	}
	t.completed, err = instruments.Int64Counter("tripgo.trips.completed", metric.WithUnit("{trip}"))
	if err != nil {
		_ = t.Shutdown(ctx)
		return nil, fmt.Errorf("create trip completed metric: %w", err)
	}

	otel.SetTracerProvider(tracer)
	otel.SetMeterProvider(meter)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	return t, nil
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	return errors.Join(t.tracer.Shutdown(ctx), t.meter.Shutdown(ctx))
}

func samplerFromEnv() (sdktrace.Sampler, error) {
	name := strings.ToLower(envOr("OTEL_TRACES_SAMPLER", "parentbased_always_on"))
	switch name {
	case "always_on":
		return sdktrace.AlwaysSample(), nil
	case "always_off":
		return sdktrace.NeverSample(), nil
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample()), nil
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample()), nil
	case "traceidratio", "parentbased_traceidratio":
		ratio, err := strconv.ParseFloat(envOr("OTEL_TRACES_SAMPLER_ARG", "1"), 64)
		if err != nil || ratio < 0 || ratio > 1 {
			return nil, errors.New("OTEL_TRACES_SAMPLER_ARG must be a number from 0 to 1")
		}
		base := sdktrace.TraceIDRatioBased(ratio)
		if name == "parentbased_traceidratio" {
			return sdktrace.ParentBased(base), nil
		}
		return base, nil
	default:
		return nil, fmt.Errorf("unsupported OTEL_TRACES_SAMPLER %q", name)
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
