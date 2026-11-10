package domain

import (
	"context"
	"time"
)

// Интерфейсы репозиториев (порты). Реализация — internal/repository/postgres.
// Репозитории переводят ошибки хранилища в ошибки домена: ErrNotFound, ErrConflict, ErrInvalidInput.

type UserRepository interface {
	Create(ctx context.Context, u *User) error // ErrConflict, если логин занят
	GetByLogin(ctx context.Context, login string) (User, error)
	GetByID(ctx context.Context, id int64) (User, error)
}

type RouteRepository interface {
	List(ctx context.Context, onlyActive bool) ([]Route, error)
	Get(ctx context.Context, id int64) (Route, error)
	Create(ctx context.Context, r *Route) error // ErrConflict, если номер занят
	Update(ctx context.Context, r *Route) error
	Delete(ctx context.Context, id int64) error // ErrConflict, если на маршрут ссылаются рейсы
	ListStops(ctx context.Context, routeID int64) ([]RouteStop, error)
	ListAllStops(ctx context.Context) ([]Stop, error)
}

type VehicleRepository interface {
	List(ctx context.Context) ([]Vehicle, error)
	Get(ctx context.Context, id int64) (Vehicle, error)
	Create(ctx context.Context, v *Vehicle) error // ErrConflict, если бортовой номер занят
	UpdateStatus(ctx context.Context, id int64, status VehicleStatus) error
}

type TripRepository interface {
	List(ctx context.Context, f TripFilter) ([]Trip, error)
	Get(ctx context.Context, id int64) (Trip, error)
	Create(ctx context.Context, t *Trip) error // ErrInvalidInput при несуществующем маршруте или автобусе
	// Transition атомарно переводит рейс из статуса from в to и проставляет фактическое время.
	// ErrNotFound — рейса нет; ErrConflict — статус уже не from или у автобуса уже есть активный рейс.
	Transition(ctx context.Context, id int64, from, to TripStatus, at time.Time) (Trip, error)
}

type IncidentRepository interface {
	List(ctx context.Context, f IncidentFilter) ([]Incident, error)
	Get(ctx context.Context, id int64) (Incident, error)
	// Resolve закрывает открытый инцидент. ErrNotFound — нет такого; ErrConflict — уже закрыт.
	Resolve(ctx context.Context, id, userID int64, at time.Time) (Incident, error)
}
