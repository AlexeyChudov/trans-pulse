package postgres_test

import (
	"errors"
	"testing"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/repository/postgres"
)

func TestRouteCrud(t *testing.T) {
	repo := postgres.NewRouteRepository(testPool(t))
	r := domain.Route{Number: "7", Name: "Центр — Вокзал", IsActive: true}
	if err := repo.Create(ctx, &r); err != nil || r.ID == 0 {
		t.Fatalf("Create: %+v, %v", r, err)
	}
	if err := repo.Create(ctx, &domain.Route{Number: "7", Name: "Дубль", IsActive: true}); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("дубликат номера: %v", err)
	}

	r.Name, r.IsActive = "Новое имя", false
	if err := repo.Update(ctx, &r); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := repo.Get(ctx, r.ID)
	if err != nil || got.Name != "Новое имя" || got.IsActive {
		t.Fatalf("Get после Update: %+v, %v", got, err)
	}
	if err := repo.Update(ctx, &domain.Route{ID: 999, Number: "x", Name: "x"}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Update несуществующего: %v", err)
	}

	if err := repo.Delete(ctx, r.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, r.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get удалённого: %v", err)
	}
	if err := repo.Delete(ctx, r.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("повторное Delete: %v", err)
	}
}

func TestRouteListOnlyActive(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewRouteRepository(pool)
	seedRoute(t, pool, "1")
	off := seedRoute(t, pool, "2")
	pool.Exec(ctx, `UPDATE routes SET is_active = FALSE WHERE id = $1`, off)

	active, err := repo.List(ctx, true)
	if err != nil || len(active) != 1 || active[0].Number != "1" {
		t.Fatalf("активные: %+v, %v", active, err)
	}
	all, _ := repo.List(ctx, false)
	if len(all) != 2 {
		t.Fatalf("все: %+v", all)
	}
}

func TestRouteListEmptyIsNotNil(t *testing.T) {
	list, err := postgres.NewRouteRepository(testPool(t)).List(ctx, true)
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("ожидался пустой непустой-nil срез, получено %#v, %v", list, err)
	}
}

func TestRouteDeleteReferencedByTripIsConflict(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewRouteRepository(pool)
	route := seedRoute(t, pool, "1")
	seedTrip(t, pool, route, seedVehicle(t, pool, "B1", "IN_SERVICE"), "PLANNED")

	if err := repo.Delete(ctx, route); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ожидалось ErrConflict, получено %v", err)
	}
}

func TestRouteStopsOrderedBySequence(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewRouteRepository(pool)
	route := seedRoute(t, pool, "1")
	a := mustID(t, pool, `INSERT INTO stops (name, latitude, longitude) VALUES ('Вокзал', 50.4419, 30.5118) RETURNING id`)
	b := mustID(t, pool, `INSERT INTO stops (name, latitude, longitude) VALUES ('Центр', 50.4501, 30.5234) RETURNING id`)
	pool.Exec(ctx, `INSERT INTO route_stops (route_id, stop_id, sequence_no) VALUES ($1, $2, 2), ($1, $3, 1)`, route, a, b)

	stops, err := repo.ListStops(ctx, route)
	if err != nil || len(stops) != 2 {
		t.Fatalf("получено %+v, %v", stops, err)
	}
	if stops[0].Name != "Центр" || stops[0].SequenceNo != 1 || stops[1].Name != "Вокзал" || stops[1].SequenceNo != 2 {
		t.Fatalf("неверный порядок: %+v", stops)
	}
	if stops[0].Latitude != 50.4501 || stops[0].Longitude != 30.5234 {
		t.Fatalf("неверные координаты: %+v", stops[0])
	}

	all, err := repo.ListAllStops(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("ListAllStops: %+v, %v", all, err)
	}
}

func TestRouteGetStop(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewRouteRepository(pool)
	id := mustID(t, pool, `INSERT INTO stops (name, latitude, longitude) VALUES ('Вокзал', 50.4419, 30.5118) RETURNING id`)

	stop, err := repo.GetStop(ctx, id)
	if err != nil || stop.Name != "Вокзал" || stop.Latitude != 50.4419 || stop.Longitude != 30.5118 {
		t.Fatalf("GetStop: %+v, %v", stop, err)
	}
	if _, err := repo.GetStop(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("несуществующая остановка: %v", err)
	}
}
