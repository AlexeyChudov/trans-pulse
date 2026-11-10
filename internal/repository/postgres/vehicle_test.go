package postgres_test

import (
	"errors"
	"testing"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/repository/postgres"
)

func TestVehicleCreateGetUpdateStatus(t *testing.T) {
	repo := postgres.NewVehicleRepository(testPool(t))
	v := domain.Vehicle{FleetNumber: "B0101", Model: "MAN", Status: domain.VehicleRegistered}
	if err := repo.Create(ctx, &v); err != nil || v.ID == 0 {
		t.Fatalf("Create: %+v, %v", v, err)
	}
	if err := repo.Create(ctx, &domain.Vehicle{FleetNumber: "B0101", Model: "X", Status: domain.VehicleRegistered}); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("дубликат номера: %v", err)
	}

	if err := repo.UpdateStatus(ctx, v.ID, domain.VehicleInService); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, err := repo.Get(ctx, v.ID)
	if err != nil || got.Status != domain.VehicleInService {
		t.Fatalf("Get: %+v, %v", got, err)
	}
	if err := repo.UpdateStatus(ctx, 999, domain.VehicleInService); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("UpdateStatus несуществующего: %v", err)
	}
	if err := repo.UpdateStatus(ctx, v.ID, "BROKEN"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("недопустимый статус (CHECK): %v", err)
	}
}

func TestVehicleListAndNotFound(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewVehicleRepository(pool)
	seedVehicle(t, pool, "B1", "IN_SERVICE")
	seedVehicle(t, pool, "B2", "RETIRED")
	list, err := repo.List(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("List: %+v, %v", list, err)
	}
	if _, err := repo.Get(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get: %v", err)
	}
}
