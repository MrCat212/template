package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/MrCat212/template/internal/domain/trip"
)

type Client struct {
	httpClient *http.Client
	url        string
}

type HTTPStatusError struct {
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("push service returned HTTP %d", e.StatusCode)
}

type requestBody struct {
	RequestID   string            `json:"request_id"`
	RecipientID uuid.UUID         `json:"recipient_id"`
	Kind        string            `json:"kind"`
	Data        map[string]string `json:"data"`
}

func New(baseURL string, timeout time.Duration) (*Client, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("push timeout must be positive")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse push URL: %w", err)
	}

	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("invalid push URL")
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/v1/push"

	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		url: parsed.String(),
	}, nil
}

func (c *Client) RequestPosition(
	ctx context.Context,
	item trip.StaleTrip,
) error {
	ctx, span := otel.Tracer("trip-service.gateway.push").Start(
		ctx,
		"push.request_position",
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	span.SetAttributes(
		attribute.String("trip.id", item.ID.String()),
		attribute.String("http.request.method", http.MethodPost),
		attribute.String("url.path", "/api/v1/push"),
	)

	body := requestBody{
		RequestID:   uuid.NewString(),
		RecipientID: item.DriverID,
		Kind:        "REQUEST_POSITION",
		Data: map[string]string{
			"trip_id": item.ID.String(),
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal push request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.url,
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("create push request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("send push request: %w", err)
	}
	defer resp.Body.Close()

	span.SetAttributes(
		attribute.Int("http.response.status_code", resp.StatusCode),
	)

	if resp.StatusCode != http.StatusAccepted {
		statusErr := &HTTPStatusError{StatusCode: resp.StatusCode}
		span.RecordError(statusErr)
		span.SetStatus(codes.Error, statusErr.Error())

		return statusErr
	}

	return nil
}

func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}
