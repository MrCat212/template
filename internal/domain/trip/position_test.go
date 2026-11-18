package trip

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestPositionValidation(t *testing.T) {
	const (
		latitudeField  = "coordinates.latitude"
		longitudeField = "coordinates.longitude"
	)

	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		latitude   float64
		longitude  float64
		recordedAt time.Time
		wantField  string
	}{
		{"valid", 55.75, 37.61, now, ""},
		{"old position", 55.75, 37.61, now.Add(-24 * time.Hour), ""},
		{"one minute ahead", 55.75, 37.61, now.Add(time.Minute), ""},
		{"latitude below minimum", -91, 37.61, now, latitudeField},
		{"latitude above maximum", 91, 37.61, now, latitudeField},
		{"longitude below minimum", 55.75, -181, now, longitudeField},
		{"longitude above maximum", 55.75, 181, now, longitudeField},
		{"latitude NaN", math.NaN(), 37.61, now, latitudeField},
		{"longitude infinity", 55.75, math.Inf(1), now, longitudeField},
		{"empty time", 55.75, 37.61, time.Time{}, "recorded_at"},
		{"future time", 55.75, 37.61, now.Add(2 * time.Minute), "recorded_at"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			position := Position{
				Latitude:   test.latitude,
				Longitude:  test.longitude,
				RecordedAt: test.recordedAt,
			}

			err := position.Validate(now)

			if test.wantField == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("expected validation error, got %v", err)
			}

			if validationErr.Field != test.wantField {
				t.Errorf("expected field %q, got %q", test.wantField, validationErr.Field)
			}
		})
	}
}
