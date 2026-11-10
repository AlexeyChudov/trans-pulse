package domain

import "time"

type VehicleStatus string

const (
	VehicleRegistered   VehicleStatus = "REGISTERED"
	VehicleInService    VehicleStatus = "IN_SERVICE"
	VehicleOutOfService VehicleStatus = "OUT_OF_SERVICE"
	VehicleRetired      VehicleStatus = "RETIRED"
)

func (s VehicleStatus) IsValid() bool {
	switch s {
	case VehicleRegistered, VehicleInService, VehicleOutOfService, VehicleRetired:
		return true
	}
	return false
}

// CanTransitionTo задаёт жизненный цикл автобуса: REGISTERED → IN_SERVICE ⇄ OUT_OF_SERVICE → RETIRED.
func (s VehicleStatus) CanTransitionTo(to VehicleStatus) bool {
	switch s {
	case VehicleRegistered:
		return to == VehicleInService
	case VehicleInService:
		return to == VehicleOutOfService || to == VehicleRetired
	case VehicleOutOfService:
		return to == VehicleInService || to == VehicleRetired
	}
	return false
}

type Vehicle struct {
	ID          int64
	FleetNumber string
	Model       string
	Status      VehicleStatus
	CreatedAt   time.Time
}
