package trip

import (
	"context"
	"fmt"
	"time"

	domain "github.com/MrCat212/template/internal/domain/trip"
	"github.com/MrCat212/template/internal/infra/postgres"
)

// FindStaleTripsBatch retrieves the next oldest stale trips without OFFSET.
// The (last_activity_at, id) pair prevents losing rows with equal timestamps.
func (r *Repository) FindStaleTripsBatch(
	ctx context.Context,
	staleAfter time.Duration,
	limit int,
	after *domain.StaleCursor,
) ([]domain.StaleTrip, error) {
	if staleAfter <= 0 || limit <= 0 {
		return nil, fmt.Errorf("staleAfter and limit must be positive")
	}
	if after != nil && after.LastActivityAt.IsZero() {
		return nil, fmt.Errorf("cursor timestamp must not be zero")
	}

	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	queryCtx = postgres.WithCollection(queryCtx, "trips")
	db := postgres.GetDBTX(queryCtx, r.pool)

	const firstPage = `
		SELECT id, driver_id, COALESCE(last_position_at, started_at) AS last_activity_at
		FROM trips
		WHERE status = 'active'
		  AND COALESCE(last_position_at, started_at)
		      < NOW() - ($1::double precision * INTERVAL '1 second')
		ORDER BY COALESCE(last_position_at, started_at), id
		LIMIT $2
	`
	const nextPage = `
		SELECT id, driver_id, COALESCE(last_position_at, started_at) AS last_activity_at
		FROM trips
		WHERE status = 'active'
		  AND COALESCE(last_position_at, started_at)
		      < NOW() - ($1::double precision * INTERVAL '1 second')
		  AND (COALESCE(last_position_at, started_at), id) > ($2::timestamptz, $3::uuid)
		ORDER BY COALESCE(last_position_at, started_at), id
		LIMIT $4
	`

	query := firstPage
	args := []any{staleAfter.Seconds(), limit}
	if after != nil {
		query = nextPage
		args = []any{staleAfter.Seconds(), after.LastActivityAt, after.ID, limit}
	}
	rows, err := db.Query(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find stale trips batch: %w", err)
	}
	defer rows.Close()

	result := make([]domain.StaleTrip, 0, limit)
	for rows.Next() {
		var item domain.StaleTrip
		if err := rows.Scan(&item.ID, &item.DriverID, &item.LastActivityAt); err != nil {
			return nil, fmt.Errorf("scan stale trip batch: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read stale trips batch: %w", err)
	}
	return result, nil
}
