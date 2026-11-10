package service

import (
	"context"
	"errors"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

type TripService struct {
	trips    domain.TripRepository
	routes   domain.RouteRepository
	vehicles domain.VehicleRepository
	now      func() time.Time
}

func NewTripService(trips domain.TripRepository, routes domain.RouteRepository, vehicles domain.VehicleRepository, now func() time.Time) *TripService {
	return &TripService{trips: trips, routes: routes, vehicles: vehicles, now: clockOrNow(now)}
}

func (s *TripService) List(ctx context.Context, f domain.TripFilter) ([]domain.Trip, error) {
	if f.Status != "" && !f.Status.IsValid() {
		return nil, invalid("неизвестный статус рейса")
	}
	f.Limit, f.Offset = NormalizePage(f.Limit, f.Offset)
	return s.trips.List(ctx, f)
}

func (s *TripService) Get(ctx context.Context, id int64) (domain.Trip, error) {
	return s.trips.Get(ctx, id)
}

// Create планирует рейс: маршрут должен быть активным, автобус — в эксплуатации.
func (s *TripService) Create(ctx context.Context, routeID, vehicleID int64, plannedStart, plannedEnd time.Time) (domain.Trip, error) {
	if !plannedEnd.After(plannedStart) {
		return domain.Trip{}, invalid("плановое окончание должно быть позже планового начала")
	}
	route, err := s.routes.Get(ctx, routeID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Trip{}, invalid("маршрут не найден")
	}
	if err != nil {
		return domain.Trip{}, err
	}
	if !route.IsActive {
		return domain.Trip{}, invalid("маршрут отключён")
	}
	if err := s.requireVehicleInService(ctx, vehicleID); err != nil {
		return domain.Trip{}, err
	}
	t := domain.Trip{RouteID: routeID, VehicleID: vehicleID, PlannedStart: plannedStart, PlannedEnd: plannedEnd, Status: domain.TripPlanned}
	if err := s.trips.Create(ctx, &t); err != nil {
		return domain.Trip{}, err
	}
	return t, nil
}

func (s *TripService) Start(ctx context.Context, id int64) (domain.Trip, error) {
	return s.transition(ctx, id, domain.TripInProgress)
}

func (s *TripService) Complete(ctx context.Context, id int64) (domain.Trip, error) {
	return s.transition(ctx, id, domain.TripCompleted)
}

func (s *TripService) Cancel(ctx context.Context, id int64) (domain.Trip, error) {
	return s.transition(ctx, id, domain.TripCancelled)
}

func (s *TripService) transition(ctx context.Context, id int64, to domain.TripStatus) (domain.Trip, error) {
	t, err := s.trips.Get(ctx, id)
	if err != nil {
		return domain.Trip{}, err
	}
	if !t.Status.CanTransitionTo(to) {
		return domain.Trip{}, conflict("рейс в статусе %s нельзя перевести в %s", t.Status, to)
	}
	if to == domain.TripInProgress {
		if err := s.requireVehicleInService(ctx, t.VehicleID); err != nil {
			return domain.Trip{}, err
		}
	}
	// Атомарность и правило «один активный рейс на автобус» обеспечивает репозиторий.
	return s.trips.Transition(ctx, id, t.Status, to, s.now())
}

func (s *TripService) requireVehicleInService(ctx context.Context, vehicleID int64) error {
	v, err := s.vehicles.Get(ctx, vehicleID)
	if errors.Is(err, domain.ErrNotFound) {
		return invalid("автобус не найден")
	}
	if err != nil {
		return err
	}
	if v.Status != domain.VehicleInService {
		return conflict("автобус в статусе %s, нужен IN_SERVICE", v.Status)
	}
	return nil
}
