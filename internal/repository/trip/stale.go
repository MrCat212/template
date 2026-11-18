package trip

import (
	"context"
	"fmt"
	"time"

	domain "github.com/MrCat212/template/internal/domain/trip"
	"github.com/MrCat212/template/internal/infra/postgres"
)

func (r *Repository) FindStaleTrips(
	ctx context.Context,
	staleAfter time.Duration,
) ([]domain.StaleTrip, error) {
	if staleAfter <= 0 {
		return nil, fmt.Errorf("staleAfter must be positive")
	}

	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	queryCtx = postgres.WithCollection(queryCtx, "trips")
	db := postgres.GetDBTX(queryCtx, r.pool)

	const query = `
		SELECT id, driver_id
		FROM trips
		WHERE status = 'active'
		  AND COALESCE(last_position_at, started_at)
		      < NOW() - ($1::double precision * INTERVAL '1 second')
		ORDER BY COALESCE(last_position_at, started_at) ASC, id ASC
	`

	rows, err := db.Query(queryCtx, query, staleAfter.Seconds())
	if err != nil {
		return nil, fmt.Errorf("find stale trips: %w", err)
	}
	defer rows.Close()

	trips := make([]domain.StaleTrip, 0)

	for rows.Next() {
		var item domain.StaleTrip

		if err := rows.Scan(&item.ID, &item.DriverID); err != nil {
			return nil, fmt.Errorf("scan stale trip: %w", err)
		}

		trips = append(trips, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read stale trips: %w", err)
	}

	return trips, nil
}
