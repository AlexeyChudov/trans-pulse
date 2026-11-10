package service

import (
	"context"
	"errors"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

const (
	maxSpeedKmh        = 250
	futureTolerance    = time.Minute
	maxHistoryPeriod   = 24 * time.Hour
	defaultHistoryRows = 1000
	maxHistoryRows     = 5000
)

type TelemetryService struct {
	repo     domain.TelemetryRepository
	vehicles domain.VehicleRepository
	now      func() time.Time
}

func NewTelemetryService(repo domain.TelemetryRepository, vehicles domain.VehicleRepository, now func() time.Time) *TelemetryService {
	return &TelemetryService{repo: repo, vehicles: vehicles, now: clockOrNow(now)}
}

// Record принимает GPS-точку. Правила: автобус существует (BR-09), широта и долгота в допустимых пределах (BR-10, BR-11),
// время измерения обязательно (BR-12) и не из будущего.
func (s *TelemetryService) Record(ctx context.Context, p domain.TelemetryPoint) error {
	switch {
	case p.Timestamp.IsZero():
		return invalid("время измерения обязательно")
	case p.Timestamp.After(s.now().Add(futureTolerance)):
		return invalid("время измерения не может быть в будущем")
	case p.Latitude < -90 || p.Latitude > 90:
		return invalid("широта должна быть от -90 до 90")
	case p.Longitude < -180 || p.Longitude > 180:
		return invalid("долгота должна быть от -180 до 180")
	case p.SpeedKmh < 0 || p.SpeedKmh > maxSpeedKmh:
		return invalid("скорость должна быть от 0 до %d км/ч", maxSpeedKmh)
	}
	if _, err := s.vehicles.Get(ctx, p.VehicleID); errors.Is(err, domain.ErrNotFound) {
		return invalid("автобус не найден")
	} else if err != nil {
		return err
	}
	return s.repo.Append(ctx, p)
}

// History возвращает историю движения автобуса за период (не более суток).
func (s *TelemetryService) History(ctx context.Context, vehicleID int64, from, to time.Time, limit int) ([]domain.TelemetryPoint, error) {
	if !to.After(from) {
		return nil, invalid("конец периода должен быть позже начала")
	}
	if to.Sub(from) > maxHistoryPeriod {
		return nil, invalid("период не должен превышать 24 часа")
	}
	if _, err := s.vehicles.Get(ctx, vehicleID); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultHistoryRows
	}
	if limit > maxHistoryRows {
		limit = maxHistoryRows
	}
	return s.repo.History(ctx, vehicleID, from, to, limit)
}
