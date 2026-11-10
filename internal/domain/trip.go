package domain

import "time"

type TripStatus string

const (
	TripPlanned    TripStatus = "PLANNED"
	TripInProgress TripStatus = "IN_PROGRESS"
	TripCompleted  TripStatus = "COMPLETED"
	TripCancelled  TripStatus = "CANCELLED"
)

func (s TripStatus) IsValid() bool {
	switch s {
	case TripPlanned, TripInProgress, TripCompleted, TripCancelled:
		return true
	}
	return false
}

// CanTransitionTo задаёт жизненный цикл рейса: PLANNED → IN_PROGRESS → COMPLETED, PLANNED → CANCELLED.
// Завершённый рейс не может снова стать активным (BR-18).
func (s TripStatus) CanTransitionTo(to TripStatus) bool {
	switch s {
	case TripPlanned:
		return to == TripInProgress || to == TripCancelled
	case TripInProgress:
		return to == TripCompleted
	}
	return false
}

type Trip struct {
	ID           int64
	RouteID      int64
	VehicleID    int64
	PlannedStart time.Time
	PlannedEnd   time.Time
	ActualStart  *time.Time
	ActualEnd    *time.Time
	Status       TripStatus
	CreatedAt    time.Time
}

type TripFilter struct {
	Status  TripStatus // пусто — любой
	RouteID int64      // 0 — любой
	Limit   int
	Offset  int
}
