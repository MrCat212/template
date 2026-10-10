package trip

import "errors"

var ErrDriverBusy = errors.New("driver already has active trip")
var ErrTripNotFound = errors.New("trip not found")
var ErrTripCompleted = errors.New("trip already completed")
