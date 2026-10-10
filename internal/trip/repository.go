package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/MrCat212/template/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewRepository(
	pool *pgxpool.Pool,
	queryTimeout time.Duration,
) *Repository {
	return &Repository{
		pool:         pool,
		queryTimeout: queryTimeout,
	}
}

func (r *Repository) Create(ctx context.Context, trip Trip) error {
	query, args, err := sq.
		Insert("trips").
		Columns(
			"id",
			"user_id",
			"driver_id",
			"start_latitude",
			"start_longitude",
			"end_latitude",
			"end_longitude",
			"price",
			"status",
			"started_at",
			"finished_at",
		).
		Values(
			trip.ID,
			trip.UserID,
			trip.DriverID,
			trip.StartLatitude,
			trip.StartLongitude,
			trip.EndLatitude,
			trip.EndLongitude,
			trip.Price,
			trip.Status,
			trip.StartedAt,
			trip.FinishedAt,
		).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("build create trip query: %w", err)
	}

	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	db := database.GetDBTX(queryCtx, r.pool)

	_, err = db.Exec(queryCtx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "trips_active_driver_unique" {
			return ErrDriverBusy
		}

		return fmt.Errorf("create trip: %w", err)
	}

	return nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (Trip, error) {
	query, args, err := sq.
		Select(
			"id",
			"user_id",
			"driver_id",
			"start_latitude",
			"start_longitude",
			"end_latitude",
			"end_longitude",
			"price",
			"status",
			"started_at",
			"finished_at",
		).
		From("trips").
		Where(sq.Eq{"id": id}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return Trip{}, fmt.Errorf("build get trip query: %w", err)
	}

	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	db := database.GetDBTX(queryCtx, r.pool)

	var trip Trip

	err = db.QueryRow(queryCtx, query, args...).Scan(
		&trip.ID,
		&trip.UserID,
		&trip.DriverID,
		&trip.StartLatitude,
		&trip.StartLongitude,
		&trip.EndLatitude,
		&trip.EndLongitude,
		&trip.Price,
		&trip.Status,
		&trip.StartedAt,
		&trip.FinishedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return Trip{}, ErrTripNotFound
	}

	if err != nil {
		return Trip{}, fmt.Errorf("get trip: %w", err)
	}

	return trip, nil
}

func (r *Repository) Finish(
	ctx context.Context,
	id uuid.UUID,
	finishedAt time.Time,
) (Trip, error) {
	query, args, err := sq.
		Update("trips").
		Set("status", StatusCompleted).
		Set("finished_at", finishedAt).
		Set("updated_at", finishedAt).
		Where(sq.Eq{
			"id":     id,
			"status": StatusActive,
		}).
		Suffix(`
			RETURNING
				id,
				user_id,
				driver_id,
				start_latitude,
				start_longitude,
				end_latitude,
				end_longitude,
				price,
				status,
				started_at,
				finished_at
		`).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return Trip{}, fmt.Errorf("build finish trip query: %w", err)
	}

	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	db := database.GetDBTX(queryCtx, r.pool)

	var trip Trip

	err = db.QueryRow(queryCtx, query, args...).Scan(
		&trip.ID,
		&trip.UserID,
		&trip.DriverID,
		&trip.StartLatitude,
		&trip.StartLongitude,
		&trip.EndLatitude,
		&trip.EndLongitude,
		&trip.Price,
		&trip.Status,
		&trip.StartedAt,
		&trip.FinishedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		existingTrip, getErr := r.GetByID(queryCtx, id)

		if errors.Is(getErr, ErrTripNotFound) {
			return Trip{}, ErrTripNotFound
		}

		if getErr != nil {
			return Trip{}, fmt.Errorf("check trip after finish: %w", getErr)
		}

		if existingTrip.Status == StatusCompleted {
			return Trip{}, ErrTripCompleted
		}
	}

	if err != nil {
		return Trip{}, fmt.Errorf("finish trip: %w", err)
	}

	return trip, nil
}

func (r *Repository) AddStatusHistory(
	ctx context.Context,
	tripID uuid.UUID,
	fromStatus *Status,
	toStatus Status,
	reason string,
	changedAt time.Time,
) error {
	query, args, err := sq.
		Insert("trip_status_history").
		Columns(
			"trip_id",
			"from_status",
			"to_status",
			"reason",
			"changed_at",
		).
		Values(
			tripID,
			fromStatus,
			toStatus,
			reason,
			changedAt,
		).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("build status history query: %w", err)
	}

	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	db := database.GetDBTX(queryCtx, r.pool)

	_, err = db.Exec(queryCtx, query, args...)
	if err != nil {
		return fmt.Errorf("add status history: %w", err)
	}

	return nil
}
