//go:build integration

package trip_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	domain "github.com/MrCat212/template/internal/domain/trip"
)

func TestFindStaleTrips(t *testing.T) {
	ctx := context.Background()
	pool, repo := setupRepository(t)

	now := time.Now().UTC()

	// Создаём поездку с заданным временем начала.
	createTrip := func(startedAt time.Time) domain.Trip {
		t.Helper()

		newTrip := makeTrip()
		newTrip.StartedAt = startedAt

		cleanupTrip(t, pool, newTrip.ID)

		if err := repo.Create(ctx, newTrip); err != nil {
			t.Fatalf("create trip: %v", err)
		}

		return newTrip
	}

	// Поездка без координат, началась 20 минут назад.
	staleWithoutPositions := createTrip(now.Add(-20 * time.Minute))

	// Поездка с последней координатой 10 минут назад.
	staleWithPositions := createTrip(now.Add(-30 * time.Minute))

	if err := repo.UpdateLastPosition(
		ctx,
		staleWithPositions.ID,
		now.Add(-10*time.Minute),
	); err != nil {
		t.Fatal(err)
	}

	// Свежая поездка, координат пока нет.
	freshWithoutPositions := createTrip(now)

	// Старая поездка, но координата только что поступила.
	freshWithPositions := createTrip(now.Add(-40 * time.Minute))

	if err := repo.UpdateLastPosition(
		ctx,
		freshWithPositions.ID,
		now,
	); err != nil {
		t.Fatal(err)
	}

	// Завершённая поездка.
	completedTrip := createTrip(now.Add(-50 * time.Minute))

	if _, err := repo.Finish(
		ctx,
		completedTrip.ID,
		now,
	); err != nil {
		t.Fatal(err)
	}

	// Ищем поездки без свежих координат дольше минуты.
	result, err := repo.FindStaleTrips(ctx, time.Minute)
	if err != nil {
		t.Fatalf("find stale trips: %v", err)
	}

	// В базе могут быть другие тестовые поездки,
	// поэтому проверяем именно созданные нами записи.
	found := make(map[uuid.UUID]int, len(result))

	for index, item := range result {
		found[item.ID] = index
	}

	// Обе устаревшие поездки должны попасть в результат.
	firstIndex, ok := found[staleWithoutPositions.ID]
	if !ok {
		t.Error("stale trip without positions was not found")
	}

	secondIndex, ok := found[staleWithPositions.ID]
	if !ok {
		t.Error("stale trip with old position was not found")
	}

	// Более старая поездка должна идти первой.
	if _, firstFound := found[staleWithoutPositions.ID]; firstFound {
		if _, secondFound := found[staleWithPositions.ID]; secondFound {
			if firstIndex >= secondIndex {
				t.Error("stale trips are not sorted by last activity")
			}
		}
	}

	// Свежие поездки не должны попадать в результат.
	if _, ok := found[freshWithoutPositions.ID]; ok {
		t.Error("fresh trip without positions was selected")
	}

	if _, ok := found[freshWithPositions.ID]; ok {
		t.Error("fresh trip with recent position was selected")
	}

	// Завершённая поездка тоже не должна попадать.
	if _, ok := found[completedTrip.ID]; ok {
		t.Error("completed trip was selected")
	}
}

func TestFindStaleTripsInvalidThreshold(t *testing.T) {
	_, repo := setupRepository(t)

	for _, duration := range []time.Duration{0, -time.Second} {
		_, err := repo.FindStaleTrips(
			context.Background(),
			duration,
		)

		if err == nil {
			t.Errorf("expected error for duration %v", duration)
		}
	}
}
