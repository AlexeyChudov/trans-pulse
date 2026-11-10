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

// ---- документные данные ----

type telemetryRequest struct {
	VehicleID int64     `json:"vehicle_id"`
	TripID    int64     `json:"trip_id"`
	Timestamp time.Time `json:"ts"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	SpeedKmh  float64   `json:"speed_kmh"`
}

type telemetryResponse struct {
	VehicleID int64     `json:"vehicle_id"`
	TripID    int64     `json:"trip_id,omitempty"`
	Timestamp time.Time `json:"ts"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	SpeedKmh  float64   `json:"speed_kmh"`
}

type journalEventRequest struct {
	At           time.Time `json:"at"`
	Type         string    `json:"type"`
	StopID       int64     `json:"stop_id"`
	DelaySeconds int       `json:"delay_seconds"`
	Note         string    `json:"note"`
}

type journalEventResponse struct {
	At           time.Time `json:"at"`
	Type         string    `json:"type"`
	StopID       int64     `json:"stop_id,omitempty"`
	DelaySeconds int       `json:"delay_seconds,omitempty"`
	Note         string    `json:"note,omitempty"`
}

type journalResponse struct {
	TripID    int64                  `json:"trip_id"`
	Events    []journalEventResponse `json:"events"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

type caseCreateRequest struct {
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

type caseUpdateRequest struct {
	Title *string   `json:"title"`
	Tags  *[]string `json:"tags"`
}

type commentRequest struct {
	Text string `json:"text"`
}

type commentResponse struct {
	AuthorID int64     `json:"author_id"`
	Text     string    `json:"text"`
	At       time.Time `json:"at"`
}

type caseResponse struct {
	IncidentID int64             `json:"incident_id"`
	TripID     int64             `json:"trip_id"`
	VehicleID  int64             `json:"vehicle_id"`
	RouteID    int64             `json:"route_id"`
	Title      string            `json:"title"`
	Tags       []string          `json:"tags"`
	Status     string            `json:"status"`
	Comments   []commentResponse `json:"comments"`
	CreatedBy  int64             `json:"created_by"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	ClosedAt   *time.Time        `json:"closed_at"`
	ClosedBy   *int64            `json:"closed_by"`
}

type stopPointCreateRequest struct {
	StopID     int64          `json:"stop_id"`
	Attributes map[string]any `json:"attributes"`
}

type stopPointAttributesRequest struct {
	Attributes map[string]any `json:"attributes"`
}

type stopPointResponse struct {
	StopID     int64          `json:"stop_id"`
	Name       string         `json:"name"`
	Latitude   float64        `json:"latitude"`
	Longitude  float64        `json:"longitude"`
	RouteIDs   []int64        `json:"route_ids"`
	Attributes map[string]any `json:"attributes"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type nearStopResponse struct {
	stopPointResponse
	DistanceM float64 `json:"distance_m"`
}

type routeStopRefDTO struct {
	StopID int64 `json:"stop_id"`
	Seq    int   `json:"seq"`
}

type routeStopsRequest struct {
	Stops []routeStopRefDTO `json:"stops"`
}

type routeProfileResponse struct {
	RouteID   int64             `json:"route_id"`
	Stops     []routeStopRefDTO `json:"stops"`
	UpdatedAt time.Time         `json:"updated_at"`
}

func toTelemetry(p domain.TelemetryPoint) telemetryResponse {
	return telemetryResponse{VehicleID: p.VehicleID, TripID: p.TripID, Timestamp: p.Timestamp, Latitude: p.Latitude, Longitude: p.Longitude, SpeedKmh: p.SpeedKmh}
}

func toJournal(j domain.TripJournal) journalResponse {
	events := mapList(j.Events, func(e domain.JournalEvent) journalEventResponse {
		return journalEventResponse{At: e.At, Type: string(e.Type), StopID: e.StopID, DelaySeconds: e.DelaySeconds, Note: e.Note}
	})
	return journalResponse{TripID: j.TripID, Events: events, CreatedAt: j.CreatedAt, UpdatedAt: j.UpdatedAt}
}

func toCase(c domain.IncidentCase) caseResponse {
	tags := c.Tags
	if tags == nil {
		tags = []string{}
	}
	return caseResponse{IncidentID: c.IncidentID, TripID: c.TripID, VehicleID: c.VehicleID, RouteID: c.RouteID, Title: c.Title, Tags: tags,
		Status: string(c.Status), Comments: mapList(c.Comments, func(cm domain.CaseComment) commentResponse {
			return commentResponse{AuthorID: cm.AuthorID, Text: cm.Text, At: cm.At}
		}), CreatedBy: c.CreatedBy, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, ClosedAt: c.ClosedAt, ClosedBy: c.ClosedBy}
}

func toStopPoint(s domain.StopPoint) stopPointResponse {
	routes, attrs := s.RouteIDs, s.Attributes
	if routes == nil {
		routes = []int64{}
	}
	if attrs == nil {
		attrs = map[string]any{}
	}
	return stopPointResponse{StopID: s.StopID, Name: s.Name, Latitude: s.Latitude, Longitude: s.Longitude, RouteIDs: routes, Attributes: attrs, UpdatedAt: s.UpdatedAt}
}

func toRouteProfile(p domain.RouteProfile) routeProfileResponse {
	return routeProfileResponse{RouteID: p.RouteID, UpdatedAt: p.UpdatedAt,
		Stops: mapList(p.Stops, func(r domain.RouteStopRef) routeStopRefDTO { return routeStopRefDTO{StopID: r.StopID, Seq: r.Seq} })}
}
