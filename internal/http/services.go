// Package httpapi — слой HTTP: chi-роутер, middleware (JWT, роли), handlers и dto.
// Зависит от сервисов только через интерфейсы, объявленные здесь (их реализует internal/service).
package httpapi

import (
	"context"
	"log/slog"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/jwt"
)

type AuthService interface {
	Register(ctx context.Context, login, password string) (domain.User, error)
	Login(ctx context.Context, login, password string) (string, domain.User, error)
	Me(ctx context.Context, userID int64) (domain.User, error)
}

type RouteService interface {
	List(ctx context.Context) ([]domain.Route, error)
	Get(ctx context.Context, id int64) (domain.Route, error)
	Create(ctx context.Context, number, name string) (domain.Route, error)
	Update(ctx context.Context, id int64, number, name string, isActive bool) (domain.Route, error)
	Delete(ctx context.Context, id int64) error
	Stops(ctx context.Context, routeID int64) ([]domain.RouteStop, error)
	AllStops(ctx context.Context) ([]domain.Stop, error)
}

type VehicleService interface {
	List(ctx context.Context) ([]domain.Vehicle, error)
	Create(ctx context.Context, fleetNumber, model string) (domain.Vehicle, error)
	SetStatus(ctx context.Context, id int64, status domain.VehicleStatus) (domain.Vehicle, error)
}

type TripService interface {
	List(ctx context.Context, f domain.TripFilter) ([]domain.Trip, error)
	Get(ctx context.Context, id int64) (domain.Trip, error)
	Create(ctx context.Context, routeID, vehicleID int64, plannedStart, plannedEnd time.Time) (domain.Trip, error)
	Start(ctx context.Context, id int64) (domain.Trip, error)
	Complete(ctx context.Context, id int64) (domain.Trip, error)
	Cancel(ctx context.Context, id int64) (domain.Trip, error)
}

type IncidentService interface {
	List(ctx context.Context, f domain.IncidentFilter) ([]domain.Incident, error)
	Resolve(ctx context.Context, id, userID int64) (domain.Incident, error)
}

// TokenParser проверяет токен доступа (реализует internal/jwt.Manager).
type TokenParser interface {
	Parse(token string) (jwt.Claims, error)
}

type Deps struct {
	Auth      AuthService
	Routes    RouteService
	Vehicles  VehicleService
	Trips     TripService
	Incidents IncidentService
	Tokens    TokenParser
	Logger    *slog.Logger
}
