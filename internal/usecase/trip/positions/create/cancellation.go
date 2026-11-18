package create

import (
	"context"

	"github.com/google/uuid"
)

// positionCanceler invalidates a push task after a position is committed.
type positionCanceler interface {
	Cancel(tripID uuid.UUID)
}

// positionMetrics records coordinates after transaction commit.
type positionMetrics interface {
	PositionSaved(ctx context.Context)
}
