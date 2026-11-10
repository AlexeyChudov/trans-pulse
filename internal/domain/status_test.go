package domain_test

import (
	"testing"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

func TestVehicleStatusTransitions(t *testing.T) {
	cases := []struct {
		from, to domain.VehicleStatus
		allowed  bool
	}{
		{domain.VehicleRegistered, domain.VehicleInService, true},
		{domain.VehicleInService, domain.VehicleOutOfService, true},
		{domain.VehicleOutOfService, domain.VehicleInService, true},
		{domain.VehicleOutOfService, domain.VehicleRetired, true},
		{domain.VehicleInService, domain.VehicleRetired, true},
		{domain.VehicleRegistered, domain.VehicleRetired, false},
		{domain.VehicleRegistered, domain.VehicleOutOfService, false},
		{domain.VehicleRetired, domain.VehicleInService, false},
		{domain.VehicleInService, domain.VehicleInService, false},
	}
	for _, c := range cases {
		if got := c.from.CanTransitionTo(c.to); got != c.allowed {
			t.Errorf("%s -> %s: получено %v, ожидалось %v", c.from, c.to, got, c.allowed)
		}
	}
}

func TestTripStatusTransitions(t *testing.T) {
	cases := []struct {
		from, to domain.TripStatus
		allowed  bool
	}{
		{domain.TripPlanned, domain.TripInProgress, true},
		{domain.TripPlanned, domain.TripCancelled, true},
		{domain.TripInProgress, domain.TripCompleted, true},
		{domain.TripInProgress, domain.TripCancelled, false},
		{domain.TripCompleted, domain.TripInProgress, false}, // BR-18
		{domain.TripCancelled, domain.TripPlanned, false},
		{domain.TripPlanned, domain.TripCompleted, false},
	}
	for _, c := range cases {
		if got := c.from.CanTransitionTo(c.to); got != c.allowed {
			t.Errorf("%s -> %s: получено %v, ожидалось %v", c.from, c.to, got, c.allowed)
		}
	}
}

func TestRoleIsValid(t *testing.T) {
	if !domain.RoleAdmin.IsValid() || !domain.RoleDispatcher.IsValid() || domain.Role("ROOT").IsValid() {
		t.Fatal("IsValid работает неверно")
	}
}
