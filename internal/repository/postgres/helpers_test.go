package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlexeyChudov/trans-pulse/internal/repository/postgres"
)

var ctx = context.Background()

// Интеграционные тесты идут против настоящей PostgreSQL. Используется отдельная переменная
// (не TEST_DATABASE_URL), чтобы случайно не направить их на БД, которую сбрасывают тесты миграций.
const envDSN = "API_TEST_DATABASE_URL"

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(envDSN)
	if dsn == "" {
		t.Skipf("%s не задан: интеграционные тесты пропущены", envDSN)
	}
	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("подключение к БД: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE incidents, trips, users, vehicles, route_stops, stops, routes RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("очистка таблиц: %v", err)
	}
	return pool
}

func mustID(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
		t.Fatalf("подготовка данных: %v", err)
	}
	return id
}

func seedRoute(t *testing.T, pool *pgxpool.Pool, number string) int64 {
	return mustID(t, pool, `INSERT INTO routes (number, name) VALUES ($1::varchar, 'Маршрут ' || $1::varchar) RETURNING id`, number)
}

func seedVehicle(t *testing.T, pool *pgxpool.Pool, fleet, status string) int64 {
	return mustID(t, pool, `INSERT INTO vehicles (fleet_number, model, status) VALUES ($1, 'Bus', $2) RETURNING id`, fleet, status)
}

func seedTrip(t *testing.T, pool *pgxpool.Pool, routeID, vehicleID int64, status string) int64 {
	start := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	return mustID(t, pool, `INSERT INTO trips (route_id, vehicle_id, planned_start, planned_end, status) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		routeID, vehicleID, start, start.Add(time.Hour), status)
}

func seedUser(t *testing.T, pool *pgxpool.Pool, login string) int64 {
	return mustID(t, pool, `INSERT INTO users (login, password_hash, role) VALUES ($1, 'hash', 'DISPATCHER') RETURNING id`, login)
}

func seedIncident(t *testing.T, pool *pgxpool.Pool, tripID int64, typ string) int64 {
	return mustID(t, pool, `INSERT INTO incidents (trip_id, type) VALUES ($1, $2) RETURNING id`, tripID, typ)
}
