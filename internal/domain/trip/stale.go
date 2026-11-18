package trip

import "github.com/google/uuid"

// StaleTrip — активная поездка, которой требуется запрос координат.
type StaleTrip struct {
	ID       uuid.UUID
	DriverID uuid.UUID
}
