package service_test

import (
	"errors"
	"testing"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/service"
)

func TestVehicleCreateStartsRegistered(t *testing.T) {
	v, err := service.NewVehicleService(newFakeVehicles()).Create(ctx, "B0101", "MAN")
	if err != nil || v.Status != domain.VehicleRegistered {
		t.Fatalf("получено %+v, %v", v, err)
	}
}

func TestVehicleCreateValidatesAndRejectsDuplicates(t *testing.T) {
	s := service.NewVehicleService(newFakeVehicles())
	if _, err := s.Create(ctx, "", "MAN"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("пустой номер: %v", err)
	}
	if _, err := s.Create(ctx, "B0101", " "); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("пустая модель: %v", err)
	}
	s.Create(ctx, "B0101", "MAN")
	if _, err := s.Create(ctx, "B0101", "MAN"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("дубликат: %v", err)
	}
}

func TestVehicleSetStatusFollowsLifecycle(t *testing.T) {
	repo := newFakeVehicles()
	v := repo.add("B1", domain.VehicleRegistered)
	s := service.NewVehicleService(repo)

	if _, err := s.SetStatus(ctx, v.ID, domain.VehicleInService); err != nil {
		t.Fatalf("REGISTERED -> IN_SERVICE должно быть разрешено: %v", err)
	}
	if _, err := s.SetStatus(ctx, v.ID, domain.VehicleRegistered); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("возврат в REGISTERED должен быть конфликтом, получено %v", err)
	}
}

func TestVehicleSetStatusRejectsUnknownStatusAndVehicle(t *testing.T) {
	repo := newFakeVehicles()
	v := repo.add("B1", domain.VehicleInService)
	s := service.NewVehicleService(repo)
	if _, err := s.SetStatus(ctx, v.ID, domain.VehicleStatus("BROKEN")); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("неизвестный статус: %v", err)
	}
	if _, err := s.SetStatus(ctx, 99, domain.VehicleInService); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("неизвестный автобус: %v", err)
	}
}
