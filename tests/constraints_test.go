package tests

import (
	"testing"
)

func TestUniqueConstraints(t *testing.T) {
	db := openDB(t)

	t.Run("route number", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()

		_, err = tx.Exec(`
			INSERT INTO routes (number, name)
			VALUES ('TEST-1', 'Test route')
		`)
		if err != nil {
			t.Fatal(err)
		}

		_, err = tx.Exec(`
			INSERT INTO routes (number, name)
			VALUES ('TEST-1', 'Duplicate route')
		`)
		if err == nil {
			t.Fatal("expected unique violation")
		}
	})

	t.Run("vehicle fleet number", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()

		_, err = tx.Exec(`
			INSERT INTO vehicles (fleet_number, model, status)
			VALUES ('TEST-1', 'Bus', 'REGISTERED')
		`)
		if err != nil {
			t.Fatal(err)
		}

		_, err = tx.Exec(`
			INSERT INTO vehicles (fleet_number, model, status)
			VALUES ('TEST-1', 'Bus', 'REGISTERED')
		`)
		if err == nil {
			t.Fatal("expected unique violation")
		}
	})

	t.Run("user login", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()

		_, err = tx.Exec(`
			INSERT INTO users (login, password_hash, role)
			VALUES ('test-user', 'hash', 'ADMIN')
		`)
		if err != nil {
			t.Fatal(err)
		}

		_, err = tx.Exec(`
			INSERT INTO users (login, password_hash, role)
			VALUES ('test-user', 'hash2', 'DISPATCHER')
		`)
		if err == nil {
			t.Fatal("expected unique violation")
		}
	})
}

func TestCheckConstraints(t *testing.T) {
	db := openDB(t)

	tests := []struct {
		name string
		sql  string
	}{
		{
			name: "invalid latitude",
			sql: `
				INSERT INTO stops (name, latitude, longitude)
				VALUES ('Test', 100, 50)
			`,
		},
		{
			name: "invalid longitude",
			sql: `
				INSERT INTO stops (name, latitude, longitude)
				VALUES ('Test', 50, 200)
			`,
		},
		{
			name: "invalid route stop sequence",
			sql: `
				INSERT INTO route_stops (route_id, stop_id, sequence_no)
				VALUES (1, 1, 0)
			`,
		},
		{
			name: "invalid vehicle status",
			sql: `
				INSERT INTO vehicles (fleet_number, model, status)
				VALUES ('TEST', 'Bus', 'INVALID')
			`,
		},
		{
			name: "invalid trip status",
			sql: `
				INSERT INTO trips (
					route_id,
					vehicle_id,
					planned_start,
					planned_end,
					status
				)
				VALUES (
					1,
					1,
					NOW(),
					NOW() + INTERVAL '1 hour',
					'INVALID'
				)
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()

			_, err = tx.Exec(tt.sql)
			if err == nil {
				t.Fatal("expected constraint violation")
			}
		})
	}
}

func TestForeignKeyConstraints(t *testing.T) {
	db := openDB(t)

	tests := []struct {
		name string
		sql  string
	}{
		{
			name: "route_stops route",
			sql: `
				INSERT INTO route_stops (route_id, stop_id, sequence_no)
				VALUES (999999, 999999, 1)
			`,
		},
		{
			name: "trip route",
			sql: `
				INSERT INTO trips (
					route_id,
					vehicle_id,
					planned_start,
					planned_end,
					status
				)
				VALUES (
					999999,
					999999,
					NOW(),
					NOW() + INTERVAL '1 hour',
					'PLANNED'
				)
			`,
		},
		{
			name: "incident trip",
			sql: `
				INSERT INTO incidents (trip_id, type)
				VALUES (999999, 'DELAY')
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()

			_, err = tx.Exec(tt.sql)
			if err == nil {
				t.Fatal("expected foreign key violation")
			}
		})
	}
}

func TestTripTimeConstraints(t *testing.T) {
	db := openDB(t)

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO routes (number, name)
		VALUES ('TEST-TIME', 'Test route')
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = tx.Exec(`
		INSERT INTO vehicles (fleet_number, model, status)
		VALUES ('TEST-TIME', 'Bus', 'REGISTERED')
	`)
	if err != nil {
		t.Fatal(err)
	}

	var routeID, vehicleID int64

	if err := tx.QueryRow(`
		SELECT id FROM routes WHERE number = 'TEST-TIME'
	`).Scan(&routeID); err != nil {
		t.Fatal(err)
	}

	if err := tx.QueryRow(`
		SELECT id FROM vehicles WHERE fleet_number = 'TEST-TIME'
	`).Scan(&vehicleID); err != nil {
		t.Fatal(err)
	}

	_, err = tx.Exec(`
		INSERT INTO trips (
			route_id,
			vehicle_id,
			planned_start,
			planned_end,
			status
		)
		VALUES (
			$1,
			$2,
			'2026-01-02 12:00:00+00',
			'2026-01-02 11:00:00+00',
			'PLANNED'
		)
	`, routeID, vehicleID)

	if err == nil {
		t.Fatal("expected planned_end > planned_start violation")
	}
}

func TestOneActiveTripPerVehicle(t *testing.T) {
	db := openDB(t)

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO routes (number, name)
		VALUES ('TEST-ACTIVE', 'Test route')
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = tx.Exec(`
		INSERT INTO vehicles (fleet_number, model, status)
		VALUES ('TEST-ACTIVE', 'Bus', 'IN_SERVICE')
	`)
	if err != nil {
		t.Fatal(err)
	}

	var routeID, vehicleID int64

	if err := tx.QueryRow(`
		SELECT id FROM routes WHERE number = 'TEST-ACTIVE'
	`).Scan(&routeID); err != nil {
		t.Fatal(err)
	}

	if err := tx.QueryRow(`
		SELECT id FROM vehicles WHERE fleet_number = 'TEST-ACTIVE'
	`).Scan(&vehicleID); err != nil {
		t.Fatal(err)
	}

	query := `
		INSERT INTO trips (
			route_id,
			vehicle_id,
			planned_start,
			planned_end,
			status
		)
		VALUES (
			$1,
			$2,
			NOW(),
			NOW() + INTERVAL '1 hour',
			'IN_PROGRESS'
		)
	`

	if _, err := tx.Exec(query, routeID, vehicleID); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Exec(query, routeID, vehicleID); err == nil {
		t.Fatal("expected only one IN_PROGRESS trip per vehicle")
	}
}

func TestIncidentResolutionConstraint(t *testing.T) {
	db := openDB(t)

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO routes (number, name)
		VALUES ('TEST-INCIDENT', 'Test route')
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = tx.Exec(`
		INSERT INTO vehicles (fleet_number, model, status)
		VALUES ('TEST-INCIDENT', 'Bus', 'REGISTERED')
	`)
	if err != nil {
		t.Fatal(err)
	}

	var routeID, vehicleID int64

	if err := tx.QueryRow(`
		SELECT id FROM routes WHERE number = 'TEST-INCIDENT'
	`).Scan(&routeID); err != nil {
		t.Fatal(err)
	}

	if err := tx.QueryRow(`
		SELECT id FROM vehicles WHERE fleet_number = 'TEST-INCIDENT'
	`).Scan(&vehicleID); err != nil {
		t.Fatal(err)
	}

	var tripID int64

	err = tx.QueryRow(`
		INSERT INTO trips (
			route_id,
			vehicle_id,
			planned_start,
			planned_end,
			status
		)
		VALUES (
			$1,
			$2,
			NOW(),
			NOW() + INTERVAL '1 hour',
			'PLANNED'
		)
		RETURNING id
	`, routeID, vehicleID).Scan(&tripID)

	if err != nil {
		t.Fatal(err)
	}

	// RESOLVED без resolved_at и resolved_by_user_id запрещён.
	_, err = tx.Exec(`
		INSERT INTO incidents (
			trip_id,
			type,
			status
		)
		VALUES ($1, 'DELAY', 'RESOLVED')
	`, tripID)

	if err == nil {
		t.Fatal("expected incident resolution constraint violation")
	}
}
