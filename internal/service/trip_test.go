package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/service"
)

var t0 = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

type tripEnv struct {
	trips    *fakeTrips
	routes   *fakeRoutes
	vehicles *fakeVehicles
	svc      *service.TripService
	route    domain.Route
	bus      domain.Vehicle
}

func newTripEnv() tripEnv {
	e := tripEnv{trips: newFakeTrips(), routes: newFakeRoutes(), vehicles: newFakeVehicles()}
	e.route = e.routes.add("1", true)
	e.bus = e.vehicles.add("B1", domain.VehicleInService)
	e.svc = service.NewTripService(e.trips, e.routes, e.vehicles, func() time.Time { return t0 })
	return e
}

func (e tripEnv) create(t *testing.T) domain.Trip {
	t.Helper()
	trip, err := e.svc.Create(ctx, e.route.ID, e.bus.ID, t0.Add(time.Hour), t0.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return trip
}

func TestTripCreateStartsPlanned(t *testing.T) {
	trip := newTripEnv().create(t)
	if trip.Status != domain.TripPlanned || trip.ID == 0 {
		t.Fatalf("получено %+v", trip)
	}
}

func TestTripCreateRejectsEndBeforeStart(t *testing.T) {
	e := newTripEnv()
	if _, err := e.svc.Create(ctx, e.route.ID, e.bus.ID, t0.Add(2*time.Hour), t0.Add(time.Hour)); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("ожидалось ErrInvalidInput, получено %v", err)
	}
}

func TestTripCreateRejectsUnknownOrInactiveRoute(t *testing.T) {
	e := newTripEnv()
	inactive := e.routes.add("2", false)
	for name, id := range map[string]int64{"нет маршрута": 99, "маршрут отключён": inactive.ID} {
		if _, err := e.svc.Create(ctx, id, e.bus.ID, t0, t0.Add(time.Hour)); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: ожидалось ErrInvalidInput, получено %v", name, err)
		}
	}
}

func TestTripCreateRejectsVehicleNotInService(t *testing.T) {
	e := newTripEnv()
	out := e.vehicles.add("B2", domain.VehicleOutOfService)
	if _, err := e.svc.Create(ctx, e.route.ID, out.ID, t0, t0.Add(time.Hour)); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("автобус не в эксплуатации: ожидалось ErrConflict, получено %v", err)
	}
	if _, err := e.svc.Create(ctx, e.route.ID, 99, t0, t0.Add(time.Hour)); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("нет автобуса: ожидалось ErrInvalidInput, получено %v", err)
	}
}

func TestTripStartSetsInProgressAndActualStart(t *testing.T) {
	e := newTripEnv()
	trip := e.create(t)
	got, err := e.svc.Start(ctx, trip.ID)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got.Status != domain.TripInProgress || got.ActualStart == nil || !got.ActualStart.Equal(t0) {
		t.Fatalf("получено %+v", got)
	}
}

func TestTripStartRequiresVehicleInService(t *testing.T) {
	e := newTripEnv()
	trip := e.create(t)
	e.vehicles.UpdateStatus(ctx, e.bus.ID, domain.VehicleOutOfService)
	if _, err := e.svc.Start(ctx, trip.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ожидалось ErrConflict, получено %v", err)
	}
}

func TestTripSecondActiveTripOfSameVehicleConflicts(t *testing.T) {
	e := newTripEnv()
	first, second := e.create(t), e.create(t)
	if _, err := e.svc.Start(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Start(ctx, second.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("BR-07: ожидалось ErrConflict, получено %v", err)
	}
}

func TestTripLifecycleRules(t *testing.T) {
	e := newTripEnv()

	planned := e.create(t)
	if _, err := e.svc.Complete(ctx, planned.ID); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("завершить рейс до старта: %v", err)
	}
	if _, err := e.svc.Cancel(ctx, planned.ID); err != nil {
		t.Errorf("отменить запланированный рейс: %v", err)
	}
	if _, err := e.svc.Start(ctx, planned.ID); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("запустить отменённый рейс: %v", err)
	}

	active := e.create(t)
	e.svc.Start(ctx, active.ID)
	if _, err := e.svc.Cancel(ctx, active.ID); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("отменить идущий рейс: %v", err)
	}
	done, err := e.svc.Complete(ctx, active.ID)
	if err != nil || done.Status != domain.TripCompleted || done.ActualEnd == nil {
		t.Errorf("завершение: %+v, %v", done, err)
	}
	if _, err := e.svc.Start(ctx, active.ID); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("BR-18: завершённый рейс нельзя запустить снова: %v", err)
	}
}

func TestTripTransitionOfUnknownTripIsNotFound(t *testing.T) {
	if _, err := newTripEnv().svc.Start(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ожидалось ErrNotFound, получено %v", err)
	}
}

func TestTripListRejectsUnknownStatusFilter(t *testing.T) {
	if _, err := newTripEnv().svc.List(ctx, domain.TripFilter{Status: "WEIRD"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("ожидалось ErrInvalidInput, получено %v", err)
	}
}
