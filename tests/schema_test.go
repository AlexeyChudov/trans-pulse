package tests

import (
	"testing"
)

func TestSchemaTables(t *testing.T) {
	db := openDB(t)

	tables := []string{
		"routes",
		"stops",
		"route_stops",
		"vehicles",
		"trips",
		"users",
		"incidents",
	}

	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			var exists bool

			err := db.QueryRow(`
				SELECT EXISTS (
					SELECT 1
					FROM information_schema.tables
					WHERE table_schema = 'public'
					  AND table_name = $1
				)
			`, table).Scan(&exists)

			if err != nil {
				t.Fatal(err)
			}

			if !exists {
				t.Fatalf("table %q does not exist", table)
			}
		})
	}
}

func TestSchemaColumns(t *testing.T) {
	db := openDB(t)

	tests := []struct {
		table    string
		column   string
		dataType string
		nullable string
	}{
		{"routes", "id", "bigint", "NO"},
		{"routes", "number", "character varying", "NO"},
		{"routes", "name", "character varying", "NO"},
		{"routes", "is_active", "boolean", "NO"},
		{"routes", "created_at", "timestamp with time zone", "NO"},

		{"stops", "id", "bigint", "NO"},
		{"stops", "name", "character varying", "NO"},
		{"stops", "latitude", "numeric", "NO"},
		{"stops", "longitude", "numeric", "NO"},

		{"route_stops", "route_id", "bigint", "NO"},
		{"route_stops", "stop_id", "bigint", "NO"},
		{"route_stops", "sequence_no", "smallint", "NO"},

		{"vehicles", "id", "bigint", "NO"},
		{"vehicles", "fleet_number", "character varying", "NO"},
		{"vehicles", "model", "character varying", "NO"},
		{"vehicles", "status", "character varying", "NO"},

		{"trips", "id", "bigint", "NO"},
		{"trips", "route_id", "bigint", "NO"},
		{"trips", "vehicle_id", "bigint", "NO"},
		{"trips", "planned_start", "timestamp with time zone", "NO"},
		{"trips", "planned_end", "timestamp with time zone", "NO"},
		{"trips", "actual_start", "timestamp with time zone", "YES"},
		{"trips", "actual_end", "timestamp with time zone", "YES"},
		{"trips", "status", "character varying", "NO"},

		{"users", "id", "bigint", "NO"},
		{"users", "login", "character varying", "NO"},
		{"users", "password_hash", "character varying", "NO"},
		{"users", "role", "character varying", "NO"},

		{"incidents", "id", "bigint", "NO"},
		{"incidents", "trip_id", "bigint", "NO"},
		{"incidents", "type", "character varying", "NO"},
		{"incidents", "status", "character varying", "NO"},
		{"incidents", "description", "text", "YES"},
		{"incidents", "resolved_at", "timestamp with time zone", "YES"},
		{"incidents", "resolved_by_user_id", "bigint", "YES"},
	}

	for _, tt := range tests {
		t.Run(tt.table+"."+tt.column, func(t *testing.T) {
			var dataType, nullable string

			err := db.QueryRow(`
				SELECT data_type, is_nullable
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name = $1
				  AND column_name = $2
			`, tt.table, tt.column).Scan(&dataType, &nullable)

			if err != nil {
				t.Fatal(err)
			}

			if dataType != tt.dataType {
				t.Errorf("type = %q, want %q", dataType, tt.dataType)
			}

			if nullable != tt.nullable {
				t.Errorf("nullable = %q, want %q", nullable, tt.nullable)
			}
		})
	}
}

func TestPrimaryKeys(t *testing.T) {
	db := openDB(t)

	tests := map[string]string{
		"routes":      "id",
		"stops":       "id",
		"vehicles":    "id",
		"trips":       "id",
		"users":       "id",
		"incidents":   "id",
		"route_stops": "route_id,stop_id",
	}

	for table, expected := range tests {
		t.Run(table, func(t *testing.T) {
			var columns string

			err := db.QueryRow(`
				SELECT string_agg(kcu.column_name, ',' ORDER BY kcu.ordinal_position)
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
				  ON tc.constraint_name = kcu.constraint_name
				 AND tc.table_schema = kcu.table_schema
				WHERE tc.table_schema = 'public'
				  AND tc.table_name = $1
				  AND tc.constraint_type = 'PRIMARY KEY'
			`, table).Scan(&columns)

			if err != nil {
				t.Fatal(err)
			}

			if columns != expected {
				t.Errorf("primary key = %q, want %q", columns, expected)
			}
		})
	}
}

func TestForeignKeys(t *testing.T) {
	db := openDB(t)

	tests := []struct {
		table     string
		column    string
		refTable  string
		refColumn string
	}{
		{"route_stops", "route_id", "routes", "id"},
		{"route_stops", "stop_id", "stops", "id"},
		{"trips", "route_id", "routes", "id"},
		{"trips", "vehicle_id", "vehicles", "id"},
		{"incidents", "trip_id", "trips", "id"},
		{"incidents", "resolved_by_user_id", "users", "id"},
	}

	for _, tt := range tests {
		t.Run(tt.table+"."+tt.column, func(t *testing.T) {
			var count int

			err := db.QueryRow(`
				SELECT COUNT(*)
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
				  ON tc.constraint_name = kcu.constraint_name
				 AND tc.table_schema = kcu.table_schema
				JOIN information_schema.constraint_column_usage ccu
				  ON tc.constraint_name = ccu.constraint_name
				 AND tc.table_schema = ccu.table_schema
				WHERE tc.table_schema = 'public'
				  AND tc.table_name = $1
				  AND kcu.column_name = $2
				  AND tc.constraint_type = 'FOREIGN KEY'
				  AND ccu.table_name = $3
				  AND ccu.column_name = $4
			`, tt.table, tt.column, tt.refTable, tt.refColumn).Scan(&count)

			if err != nil {
				t.Fatal(err)
			}

			if count != 1 {
				t.Fatalf(
					"foreign key %s.%s -> %s.%s not found",
					tt.table,
					tt.column,
					tt.refTable,
					tt.refColumn,
				)
			}
		})
	}
}

func TestRequiredIndexes(t *testing.T) {
	db := openDB(t)

	indexes := []string{
		"uq_trips_one_active_per_vehicle",
		"uq_incidents_one_open_per_type",
		"idx_trips_route_status",
		"idx_trips_vehicle_status",
		"idx_incidents_trip_status",
	}

	for _, index := range indexes {
		t.Run(index, func(t *testing.T) {
			var exists bool

			err := db.QueryRow(`
				SELECT EXISTS (
					SELECT 1
					FROM pg_indexes
					WHERE schemaname = 'public'
					  AND indexname = $1
				)
			`, index).Scan(&exists)

			if err != nil {
				t.Fatal(err)
			}

			if !exists {
				t.Fatalf("index %q does not exist", index)
			}
		})
	}
}
