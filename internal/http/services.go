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

type TelemetryService interface {
	Record(ctx context.Context, p domain.TelemetryPoint) error
	History(ctx context.Context, vehicleID int64, from, to time.Time, limit int) ([]domain.TelemetryPoint, error)
}

type JournalService interface {
	Create(ctx context.Context, tripID int64) (domain.TripJournal, error)
	Get(ctx context.Context, tripID int64) (domain.TripJournal, error)
	AddEvent(ctx context.Context, tripID int64, e domain.JournalEvent) (domain.TripJournal, error)
	Delete(ctx context.Context, tripID int64) error
}

type CaseService interface {
	Create(ctx context.Context, incidentID, userID int64, title string, tags []string) (domain.IncidentCase, error)
	Get(ctx context.Context, incidentID int64) (domain.IncidentCase, error)
	Update(ctx context.Context, incidentID int64, title *string, tags *[]string) (domain.IncidentCase, error)
	AddComment(ctx context.Context, incidentID, authorID int64, text string) (domain.IncidentCase, error)
	Close(ctx context.Context, incidentID, userID int64) (domain.IncidentCase, error)
	Delete(ctx context.Context, incidentID int64) error
}

type GeoService interface {
	CreateStopPoint(ctx context.Context, stopID int64, attrs map[string]any) (domain.StopPoint, error)
	GetStopPoint(ctx context.Context, stopID int64) (domain.StopPoint, error)
	SetAttributes(ctx context.Context, stopID int64, attrs map[string]any) (domain.StopPoint, error)
	DeleteStopPoint(ctx context.Context, stopID int64) error
	Near(ctx context.Context, lat, lon float64, radiusM, limit int) ([]domain.NearStop, error)
	SetRouteStops(ctx context.Context, routeID int64, refs []domain.RouteStopRef) (domain.RouteProfile, error)
	GetRouteProfile(ctx context.Context, routeID int64) (domain.RouteProfile, error)
	DeleteRouteProfile(ctx context.Context, routeID int64) error
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
	Telemetry TelemetryService
	Journals  JournalService
	Cases     CaseService
	Geo       GeoService
	Tokens    TokenParser
	Logger    *slog.Logger
}
