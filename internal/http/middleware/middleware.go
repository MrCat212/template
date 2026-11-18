package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"

	"github.com/MrCat212/template/internal/http/domainerror"
)

type statusWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.written {
		return
	}
	w.written = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func Route(r *http.Request) string {
	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		if pattern := routeContext.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	path := r.URL.Path
	switch path {
	case "/health", "/ready", "/api/v1/trips":
		return path
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "trips" {
		if len(parts) == 4 {
			return "/api/v1/trips/{tripId}"
		}
		if len(parts) == 5 && parts[4] == "finish" {
			return "/api/v1/trips/{tripId}/finish"
		}
	}
	return "unmatched"
}

func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if value := recover(); value != nil {
					logger.ErrorContext(r.Context(), "panic recovered", "panic", fmt.Sprint(value), "stack", string(debug.Stack()))
					domainerror.Write(w, r, errors.New("panic recovered"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func Tracing() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		observed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			defer func() {
				panicValue := recover()
				if panicValue != nil && !recorder.written {
					recorder.status = http.StatusInternalServerError
				}
				route := Route(r)
				span := trace.SpanFromContext(r.Context())
				span.SetName(r.Method + " " + route)
				span.SetAttributes(
					attribute.String("http.route", route),
					attribute.Int("http.response.status_code", recorder.status),
				)
				if recorder.status >= http.StatusInternalServerError {
					span.SetStatus(codes.Error, http.StatusText(recorder.status))
					span.SetAttributes(attribute.String("error.type", "http.server"))
				}
				if panicValue != nil {
					panic(panicValue)
				}
			}()
			next.ServeHTTP(recorder, r)
		})
		return otelhttp.NewHandler(observed, "http.server",
			otelhttp.WithMeterProvider(noop.NewMeterProvider()),
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return r.Method + " " + Route(r)
			}),
		)
	}
}

func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			defer func() {
				panicValue := recover()
				if panicValue != nil && !recorder.written {
					recorder.status = http.StatusInternalServerError
				}
				level := slog.LevelInfo
				if recorder.status >= http.StatusInternalServerError {
					level = slog.LevelError
				}
				logger.LogAttrs(r.Context(), level, "request handled",
					slog.String("method", r.Method),
					slog.String("route", Route(r)),
					slog.Int("status", recorder.status),
					slog.Float64("duration_ms", float64(time.Since(started).Microseconds())/1e3),
				)
				if panicValue != nil {
					panic(panicValue)
				}
			}()
			next.ServeHTTP(recorder, r)
		})
	}
}

type httpMetrics interface {
	HTTPStarted(ctx context.Context, method, route string)
	HTTPFinished(ctx context.Context, method, route string, status int, elapsed time.Duration)
}

func Metrics(metrics httpMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			route := Route(r)
			metrics.HTTPStarted(r.Context(), r.Method, route)
			recorder := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			defer func() {
				panicValue := recover()
				if panicValue != nil && !recorder.written {
					recorder.status = http.StatusInternalServerError
				}
				metrics.HTTPFinished(r.Context(), r.Method, route, recorder.status, time.Since(started))
				if panicValue != nil {
					panic(panicValue)
				}
			}()
			next.ServeHTTP(recorder, r)
		})
	}
}
