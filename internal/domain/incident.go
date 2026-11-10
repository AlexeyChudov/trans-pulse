package domain

import "time"

type IncidentStatus string

const (
	IncidentOpen     IncidentStatus = "OPEN"
	IncidentResolved IncidentStatus = "RESOLVED"
)

func (s IncidentStatus) IsValid() bool { return s == IncidentOpen || s == IncidentResolved }

type Incident struct {
	ID               int64
	TripID           int64
	Type             string // DELAY | NO_TELEMETRY
	Status           IncidentStatus
	Description      *string
	CreatedAt        time.Time
	ResolvedAt       *time.Time
	ResolvedByUserID *int64
}

type IncidentFilter struct {
	Status IncidentStatus // пусто — любой
	TripID int64          // 0 — любой
	Limit  int
	Offset int
}
