package trip

import (
	"time"

	"github.com/google/uuid"
)

// StaleTrip — активная поездка, которой требуется запрос координат.
type StaleTrip struct {
	ID             uuid.UUID
	DriverID       uuid.UUID
	LastActivityAt time.Time
}

// StaleCursor marks the last returned row of a keyset-paginated scan.
// The timestamp and UUID together form a unique, stable ordering key.
type StaleCursor struct {
	LastActivityAt time.Time
	ID             uuid.UUID
}
