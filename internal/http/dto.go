package httpapi

import (
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

// ---- запросы ----

type registerRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type routeRequest struct {
	Number   string `json:"number"`
	Name     string `json:"name"`
	IsActive *bool  `json:"is_active"` // обязателен при PUT, при POST не используется
}

type vehicleRequest struct {
	FleetNumber string `json:"fleet_number"`
	Model       string `json:"model"`
}

type vehicleStatusRequest struct {
	Status string `json:"status"`
}

type tripRequest struct {
	RouteID      int64     `json:"route_id"`
	VehicleID    int64     `json:"vehicle_id"`
	PlannedStart time.Time `json:"planned_start"`
	PlannedEnd   time.Time `json:"planned_end"`
}

// ---- ответы ----

type userResponse struct {
	ID        int64     `json:"id"`
	Login     string    `json:"login"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type loginResponse struct {
	Token string       `json:"token"`
	User  userResponse `json:"user"`
}

type routeResponse struct {
	ID        int64     `json:"id"`
	Number    string    `json:"number"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type stopResponse struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type routeStopResponse struct {
	stopResponse
	SequenceNo int `json:"sequence_no"`
}

type vehicleResponse struct {
	ID          int64     `json:"id"`
	FleetNumber string    `json:"fleet_number"`
	Model       string    `json:"model"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type tripResponse struct {
	ID           int64      `json:"id"`
	RouteID      int64      `json:"route_id"`
	VehicleID    int64      `json:"vehicle_id"`
	PlannedStart time.Time  `json:"planned_start"`
	PlannedEnd   time.Time  `json:"planned_end"`
	ActualStart  *time.Time `json:"actual_start"`
	ActualEnd    *time.Time `json:"actual_end"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
}

type incidentResponse struct {
	ID               int64      `json:"id"`
	TripID           int64      `json:"trip_id"`
	Type             string     `json:"type"`
	Status           string     `json:"status"`
	Description      *string    `json:"description"`
	CreatedAt        time.Time  `json:"created_at"`
	ResolvedAt       *time.Time `json:"resolved_at"`
	ResolvedByUserID *int64     `json:"resolved_by_user_id"`
}

func toUser(u domain.User) userResponse {
	return userResponse{ID: u.ID, Login: u.Login, Role: string(u.Role), IsActive: u.IsActive, CreatedAt: u.CreatedAt}
}

func toRoute(r domain.Route) routeResponse {
	return routeResponse{ID: r.ID, Number: r.Number, Name: r.Name, IsActive: r.IsActive, CreatedAt: r.CreatedAt}
}

func toStop(s domain.Stop) stopResponse {
	return stopResponse{ID: s.ID, Name: s.Name, Latitude: s.Latitude, Longitude: s.Longitude}
}

func toRouteStop(s domain.RouteStop) routeStopResponse {
	return routeStopResponse{stopResponse: toStop(s.Stop), SequenceNo: s.SequenceNo}
}

func toVehicle(v domain.Vehicle) vehicleResponse {
	return vehicleResponse{ID: v.ID, FleetNumber: v.FleetNumber, Model: v.Model, Status: string(v.Status), CreatedAt: v.CreatedAt}
}

func toTrip(t domain.Trip) tripResponse {
	return tripResponse{ID: t.ID, RouteID: t.RouteID, VehicleID: t.VehicleID, PlannedStart: t.PlannedStart, PlannedEnd: t.PlannedEnd,
		ActualStart: t.ActualStart, ActualEnd: t.ActualEnd, Status: string(t.Status), CreatedAt: t.CreatedAt}
}

func toIncident(i domain.Incident) incidentResponse {
	return incidentResponse{ID: i.ID, TripID: i.TripID, Type: i.Type, Status: string(i.Status), Description: i.Description,
		CreatedAt: i.CreatedAt, ResolvedAt: i.ResolvedAt, ResolvedByUserID: i.ResolvedByUserID}
}

// mapList преобразует срез; результат никогда не nil, чтобы в JSON получался [] а не null.
func mapList[T, R any](in []T, f func(T) R) []R {
	out := make([]R, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}
