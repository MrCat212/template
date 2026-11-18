package trip

import (
	"math"
	"time"

	"github.com/google/uuid"
)

type Position struct {
	ID         int64
	TripID     uuid.UUID
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
}

func (p Position) Validate(now time.Time) error {
	if math.IsNaN(p.Latitude) || math.IsInf(p.Latitude, 0) ||
		p.Latitude < -90 || p.Latitude > 90 {
		return &ValidationError{
			Field:  "coordinates.latitude",
			Reason: "must be between -90 and 90",
		}
	}

	if math.IsNaN(p.Longitude) || math.IsInf(p.Longitude, 0) ||
		p.Longitude < -180 || p.Longitude > 180 {
		return &ValidationError{
			Field:  "coordinates.longitude",
			Reason: "must be between -180 and 180",
		}
	}

	if p.RecordedAt.IsZero() {
		return &ValidationError{
			Field:  "recorded_at",
			Reason: "must not be empty",
		}
	}

	if p.RecordedAt.After(now.Add(time.Minute)) {
		return &ValidationError{
			Field:  "recorded_at",
			Reason: "must not be more than one minute in the future",
		}
	}

	return nil
}
