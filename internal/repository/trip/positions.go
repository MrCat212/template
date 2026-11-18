package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domain "github.com/MrCat212/template/internal/domain/trip"
	"github.com/MrCat212/template/internal/infra/postgres"
)

func (r *Repository) SavePosition(
	ctx context.Context,
	position domain.Position,
) (domain.Position, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	queryCtx = postgres.WithCollection(queryCtx, "trip_positions")
	db := postgres.GetDBTX(queryCtx, r.pool)

	const query = `
		INSERT INTO trip_positions (
			trip_id, latitude, longitude, recorded_at
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	err := db.QueryRow(
		queryCtx,
		query,
		position.TripID,
		position.Latitude,
		position.Longitude,
		position.RecordedAt,
	).Scan(&position.ID)
	if err != nil {
		return domain.Position{}, fmt.Errorf("save position: %w", err)
	}

	return position, nil
}

func (r *Repository) UpdateLastPosition(
	ctx context.Context,
	tripID uuid.UUID,
	recordedAt time.Time,
) error {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	queryCtx = postgres.WithCollection(queryCtx, "trips")
	db := postgres.GetDBTX(queryCtx, r.pool)

	const query = `
		UPDATE trips
		SET last_position_at = GREATEST(
			COALESCE(last_position_at, $2),
			$2
		),
		updated_at = NOW()
		WHERE id = $1 AND status = 'active'
	`

	result, err := db.Exec(queryCtx, query, tripID, recordedAt)
	if err != nil {
		return fmt.Errorf("update last position: %w", err)
	}

	if result.RowsAffected() == 0 {
		existing, getErr := r.GetByID(queryCtx, tripID)
		if getErr != nil {
			return getErr
		}

		if existing.Status == domain.StatusCompleted {
			return domain.ErrTripCompleted
		}

		return fmt.Errorf("update last position: active trip was not updated")
	}

	return nil
}

func (r *Repository) ListPositions(
	ctx context.Context,
	tripID uuid.UUID,
) ([]domain.Position, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	if _, err := r.GetByID(queryCtx, tripID); err != nil {
		return nil, err
	}

	queryCtx = postgres.WithCollection(queryCtx, "trip_positions")
	db := postgres.GetDBTX(queryCtx, r.pool)

	const query = `
		SELECT id, trip_id, latitude, longitude, recorded_at
		FROM trip_positions
		WHERE trip_id = $1
		ORDER BY recorded_at ASC, id ASC
	`

	rows, err := db.Query(queryCtx, query, tripID)
	if err != nil {
		return nil, fmt.Errorf("list positions: %w", err)
	}
	defer rows.Close()

	positions := make([]domain.Position, 0)

	for rows.Next() {
		var position domain.Position

		if err := rows.Scan(
			&position.ID,
			&position.TripID,
			&position.Latitude,
			&position.Longitude,
			&position.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan position: %w", err)
		}

		positions = append(positions, position)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read positions: %w", err)
	}

	return positions, nil
}
