package postgres

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type (
	collectionKey struct{}
	queryKey      struct{}
)

type queryDetails struct {
	started    time.Time
	operation  string
	collection string
}

type queryTracer struct {
	*otelpgx.Tracer
	duration metric.Float64Histogram
}

func WithCollection(ctx context.Context, collection string) context.Context {
	return context.WithValue(ctx, collectionKey{}, collection)
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	started := time.Now()
	collection, _ := ctx.Value(collectionKey{}).(string)
	operation := otelpgx.SQLOperationName(ctx, data.SQL)

	ctx = t.Tracer.TraceQueryStart(ctx, conn, data)
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("db.operation.name", operation))
	if collection != "" {
		span.SetAttributes(attribute.String("db.collection.name", collection))
	}

	return context.WithValue(ctx, queryKey{}, queryDetails{
		started: started, operation: operation, collection: collection,
	})
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	info, _ := ctx.Value(queryKey{}).(queryDetails)
	attrs := []attribute.KeyValue{
		attribute.String("db.system.name", "postgresql"),
		attribute.String("db.operation.name", info.operation),
	}
	if info.collection != "" {
		attrs = append(attrs, attribute.String("db.collection.name", info.collection))
	}
	if data.Err != nil {
		kind := databaseErrorType(data.Err)
		attrs = append(attrs, attribute.String("error.type", kind))
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(attribute.String("error.type", kind))
		var pgErr *pgconn.PgError
		if errors.As(data.Err, &pgErr) {
			span.SetAttributes(attribute.String("db.response.status_code", pgErr.Code))
		}
	}

	t.Tracer.TraceQueryEnd(ctx, conn, data)
	if !info.started.IsZero() {
		t.duration.Record(ctx, time.Since(info.started).Seconds(), metric.WithAttributes(attrs...))
	}
}

func databaseErrorType(err error) string {
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr):
		return pgErr.Code
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	default:
		return reflect.TypeOf(err).String()
	}
}
