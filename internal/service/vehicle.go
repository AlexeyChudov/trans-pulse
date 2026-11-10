package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

type VehicleService struct{ repo domain.VehicleRepository }

func NewVehicleService(repo domain.VehicleRepository) *VehicleService {
	return &VehicleService{repo: repo}
}

func (s *VehicleService) List(ctx context.Context) ([]domain.Vehicle, error) {
	return s.repo.List(ctx)
}

// Create регистрирует автобус в статусе REGISTERED.
func (s *VehicleService) Create(ctx context.Context, fleetNumber, model string) (domain.Vehicle, error) {
	fleetNumber, model = strings.TrimSpace(fleetNumber), strings.TrimSpace(model)
	if n := utf8.RuneCountInString(fleetNumber); n < 1 || n > 30 {
		return domain.Vehicle{}, invalid("бортовой номер: 1–30 символов")
	}
	if n := utf8.RuneCountInString(model); n < 1 || n > 100 {
		return domain.Vehicle{}, invalid("модель: 1–100 символов")
	}
	v := domain.Vehicle{FleetNumber: fleetNumber, Model: model, Status: domain.VehicleRegistered}
	if err := s.repo.Create(ctx, &v); err != nil {
		return domain.Vehicle{}, err
	}
	return v, nil
}

// SetStatus меняет статус автобуса по правилам жизненного цикла.
func (s *VehicleService) SetStatus(ctx context.Context, id int64, status domain.VehicleStatus) (domain.Vehicle, error) {
	if !status.IsValid() {
		return domain.Vehicle{}, invalid("неизвестный статус автобуса")
	}
	v, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Vehicle{}, err
	}
	if !v.Status.CanTransitionTo(status) {
		return domain.Vehicle{}, conflict("автобус в статусе %s нельзя перевести в %s", v.Status, status)
	}
	if err := s.repo.UpdateStatus(ctx, id, status); err != nil {
		return domain.Vehicle{}, err
	}
	v.Status = status
	return v, nil
}
