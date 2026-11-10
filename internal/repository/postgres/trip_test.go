package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/repository/postgres"
)

var tripStart = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

func TestTripCreateAndGet(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewTripRepository(pool)
	route, bus := seedRoute(t, pool, "1"), seedVehicle(t, pool, "B1", "IN_SERVICE")

	trip := domain.Trip{RouteID: route, VehicleID: bus, PlannedStart: tripStart, PlannedEnd: tripStart.Add(time.Hour), Status: domain.TripPlanned}
	if err := repo.Create(ctx, &trip); err != nil || trip.ID == 0 || trip.CreatedAt.IsZero() {
		t.Fatalf("Create: %+v, %v", trip, err)
	}
	got, err := repo.Get(ctx, trip.ID)
	if err != nil || got.Status != domain.TripPlanned || got.ActualStart != nil || !got.PlannedStart.Equal(tripStart) {
		t.Fatalf("Get: %+v, %v", got, err)
	}
	if _, err := repo.Get(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get несуществующего: %v", err)
	}
}

func TestTripCreateWithMissingReferencesIsInvalidInput(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewTripRepository(pool)
	bus := seedVehicle(t, pool, "B1", "IN_SERVICE")
	err := repo.Create(ctx, &domain.Trip{RouteID: 999, VehicleID: bus, PlannedStart: tripStart, PlannedEnd: tripStart.Add(time.Hour), Status: domain.TripPlanned})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("ожидалось ErrInvalidInput, получено %v", err)
	}
}

func TestTripTransitionSetsActualTimes(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewTripRepository(pool)
	id := seedTrip(t, pool, seedRoute(t, pool, "1"), seedVehicle(t, pool, "B1", "IN_SERVICE"), "PLANNED")

	started, err := repo.Transition(ctx, id, domain.TripPlanned, domain.TripInProgress, tripStart)
	if err != nil || started.Status != domain.TripInProgress || started.ActualStart == nil || !started.ActualStart.Equal(tripStart) || started.ActualEnd != nil {
		t.Fatalf("старт: %+v, %v", started, err)
	}
	end := tripStart.Add(45 * time.Minute)
	done, err := repo.Transition(ctx, id, domain.TripInProgress, domain.TripCompleted, end)
	if err != nil || done.Status != domain.TripCompleted || done.ActualEnd == nil || !done.ActualEnd.Equal(end) || done.ActualStart == nil {
		t.Fatalf("завершение: %+v, %v", done, err)
	}
}

func TestTripTransitionWithWrongCurrentStatusIsConflict(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewTripRepository(pool)
	id := seedTrip(t, pool, seedRoute(t, pool, "1"), seedVehicle(t, pool, "B1", "IN_SERVICE"), "COMPLETED")

	if _, err := repo.Transition(ctx, id, domain.TripPlanned, domain.TripInProgress, tripStart); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ожидалось ErrConflict, получено %v", err)
	}
	if _, err := repo.Transition(ctx, 999, domain.TripPlanned, domain.TripInProgress, tripStart); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("несуществующий рейс: ожидалось ErrNotFound, получено %v", err)
	}
}

func TestTripSecondActiveTripOfSameVehicleIsConflict(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewTripRepository(pool)
	route, bus := seedRoute(t, pool, "1"), seedVehicle(t, pool, "B1", "IN_SERVICE")
	first, second := seedTrip(t, pool, route, bus, "PLANNED"), seedTrip(t, pool, route, bus, "PLANNED")

	if _, err := repo.Transition(ctx, first, domain.TripPlanned, domain.TripInProgress, tripStart); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Transition(ctx, second, domain.TripPlanned, domain.TripInProgress, tripStart); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("BR-07 (частичный уникальный индекс): ожидалось ErrConflict, получено %v", err)
	}
}

func TestTripListFiltersAndPaging(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewTripRepository(pool)
	r1, r2 := seedRoute(t, pool, "1"), seedRoute(t, pool, "2")
	bus := seedVehicle(t, pool, "B1", "IN_SERVICE")
	seedTrip(t, pool, r1, bus, "COMPLETED")
	seedTrip(t, pool, r1, bus, "PLANNED")
	seedTrip(t, pool, r2, bus, "PLANNED")

	planned, err := repo.List(ctx, domain.TripFilter{Status: domain.TripPlanned, Limit: 10})
	if err != nil || len(planned) != 2 {
		t.Fatalf("по статусу: %+v, %v", planned, err)
	}
	byRoute, _ := repo.List(ctx, domain.TripFilter{RouteID: r1, Limit: 10})
	if len(byRoute) != 2 {
		t.Fatalf("по маршруту: %+v", byRoute)
	}
	page, _ := repo.List(ctx, domain.TripFilter{Limit: 2, Offset: 2})
	if len(page) != 1 {
		t.Fatalf("страница: %+v", page)
	}
	all, _ := repo.List(ctx, domain.TripFilter{Limit: 10})
	if len(all) != 3 || all[0].ID < all[1].ID {
		t.Fatalf("ожидалось 3 рейса от новых к старым: %+v", all)
	}
}
