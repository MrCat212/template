package convert

import (
	domain "github.com/MrCat212/template/internal/domain/trip"
	api "github.com/MrCat212/template/internal/generated"
)

func ToAPITrip(t domain.Trip) api.Trip {
	return api.Trip{
		Id: t.ID, UserId: t.UserID, DriverId: t.DriverID,
		StartPoint: api.Coordinates{Latitude: t.StartLatitude, Longitude: t.StartLongitude},
		EndPoint:   api.Coordinates{Latitude: t.EndLatitude, Longitude: t.EndLongitude},
		Price:      t.Price, Status: api.TripStatus(t.Status),
		StartedAt: t.StartedAt, FinishedAt: t.FinishedAt,
	}
}
