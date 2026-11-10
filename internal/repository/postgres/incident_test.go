package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/repository/postgres"
)

type incidentEnv struct {
	pool       *pgxpool.Pool
	repo       *postgres.IncidentRepository
	tripID     int64
	userID     int64
	incidentID int64
}

func newIncidentEnv(t *testing.T) incidentEnv {
	t.Helper()
	pool := testPool(t)
	trip := seedTrip(t, pool, seedRoute(t, pool, "1"), seedVehicle(t, pool, "B1", "IN_SERVICE"), "IN_PROGRESS")
	return incidentEnv{
		pool:       pool,
		repo:       postgres.NewIncidentRepository(pool),
		tripID:     trip,
		userID:     seedUser(t, pool, "dispatcher"),
		incidentID: seedIncident(t, pool, trip, "DELAY"),
	}
}

func TestIncidentResolveOpen(t *testing.T) {
	e := newIncidentEnv(t)
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

	got, err := e.repo.Resolve(ctx, e.incidentID, e.userID, at)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Status != domain.IncidentResolved || got.ResolvedByUserID == nil || *got.ResolvedByUserID != e.userID || got.ResolvedAt == nil || !got.ResolvedAt.Equal(at) {
		t.Fatalf("получено %+v", got)
	}
}

func TestIncidentResolveErrors(t *testing.T) {
	e := newIncidentEnv(t)
	if _, err := e.repo.Resolve(ctx, e.incidentID, e.userID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := e.repo.Resolve(ctx, e.incidentID, e.userID, time.Now()); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("повторное закрытие: %v", err)
	}
	if _, err := e.repo.Resolve(ctx, 999, e.userID, time.Now()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("несуществующий инцидент: %v", err)
	}
}

func TestIncidentResolveByUnknownUserIsInvalidInput(t *testing.T) {
	e := newIncidentEnv(t)
	if _, err := e.repo.Resolve(ctx, e.incidentID, 999, time.Now()); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("ожидалось ErrInvalidInput, получено %v", err)
	}
}

func TestIncidentListFilters(t *testing.T) {
	e := newIncidentEnv(t)
	second := seedIncident(t, e.pool, e.tripID, "NO_TELEMETRY") // другой тип: допустим второй открытый инцидент
	if _, err := e.repo.Resolve(ctx, e.incidentID, e.userID, time.Now()); err != nil {
		t.Fatal(err)
	}

	open, err := e.repo.List(ctx, domain.IncidentFilter{Status: domain.IncidentOpen, Limit: 10})
	if err != nil || len(open) != 1 || open[0].ID != second {
		t.Fatalf("открытые: %+v, %v", open, err)
	}
	resolved, err := e.repo.List(ctx, domain.IncidentFilter{Status: domain.IncidentResolved, TripID: e.tripID, Limit: 10})
	if err != nil || len(resolved) != 1 || resolved[0].ID != e.incidentID {
		t.Fatalf("закрытые: %+v, %v", resolved, err)
	}
	all, _ := e.repo.List(ctx, domain.IncidentFilter{Limit: 10})
	if len(all) != 2 || all[0].ID < all[1].ID {
		t.Fatalf("ожидалось 2 инцидента от новых к старым: %+v", all)
	}
	if _, err := e.repo.Get(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get несуществующего: %v", err)
	}
}
