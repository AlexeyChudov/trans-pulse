#!/bin/bash

set -e

DB_URL="${TEST_DATABASE_URL:?TEST_DATABASE_URL is not set}"

echo "==> Reset database"

psql "$DB_URL" <<EOF
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
EOF

echo "==> Apply version 1: init"

goose -dir ./migrations postgres "$DB_URL" up-to 20260920130922

echo "==> Insert test data"

psql "$DB_URL" <<'EOF'
INSERT INTO routes (number, name, is_active)
VALUES
    ('10', 'Центральный маршрут', TRUE),
    ('20', 'Северный маршрут', TRUE);

INSERT INTO stops (name, latitude, longitude, is_active)
VALUES
    ('Площадь Ленина', 55.030204, 82.920430, TRUE),
    ('Речной вокзал', 55.008352, 82.935732, TRUE),
    ('Вокзал', 55.041153, 82.891921, TRUE);

INSERT INTO route_stops (route_id, stop_id, sequence_no)
SELECT r.id, s.id, x.sequence_no
FROM (
    VALUES
        ('10', 'Площадь Ленина', 1),
        ('10', 'Речной вокзал', 2),
        ('20', 'Вокзал', 1),
        ('20', 'Площадь Ленина', 2)
) AS x(route_number, stop_name, sequence_no)
JOIN routes r ON r.number = x.route_number
JOIN stops s ON s.name = x.stop_name;

INSERT INTO vehicles (fleet_number, model, status)
VALUES
    ('A-001', 'ЛиАЗ-5292', 'REGISTERED'),
    ('A-002', 'ПАЗ-3204', 'IN_SERVICE');

INSERT INTO users (login, password_hash, role, is_active)
VALUES
    ('admin', 'test-password-hash', 'ADMIN', TRUE),
    ('dispatcher', 'test-password-hash', 'DISPATCHER', TRUE);

INSERT INTO trips (
    route_id,
    vehicle_id,
    planned_start,
    planned_end,
    status
)
SELECT
    r.id,
    v.id,
    '2026-09-22 08:00:00+07',
    '2026-09-22 09:00:00+07',
    'PLANNED'
FROM routes r
JOIN vehicles v ON v.fleet_number = 'A-001'
WHERE r.number = '10';
EOF

echo "==> Verify data before upgrade"

routes_before=$(psql "$DB_URL" -tAc "SELECT COUNT(*) FROM routes")
stops_before=$(psql "$DB_URL" -tAc "SELECT COUNT(*) FROM stops")
route_stops_before=$(psql "$DB_URL" -tAc "SELECT COUNT(*) FROM route_stops")
vehicles_before=$(psql "$DB_URL" -tAc "SELECT COUNT(*) FROM vehicles")
users_before=$(psql "$DB_URL" -tAc "SELECT COUNT(*) FROM users")
trips_before=$(psql "$DB_URL" -tAc "SELECT COUNT(*) FROM trips")

echo "routes:      $routes_before"
echo "stops:       $stops_before"
echo "route_stops: $route_stops_before"
echo "vehicles:    $vehicles_before"
echo "users:       $users_before"
echo "trips:       $trips_before"

echo "==> Apply version 2: integrity_indexes"

goose -dir ./migrations postgres "$DB_URL" up

echo "==> Verify data after upgrade"

routes_after=$(psql "$DB_URL" -tAc "SELECT COUNT(*) FROM routes")
stops_after=$(psql "$DB_URL" -tAc "SELECT COUNT(*) FROM stops")
route_stops_after=$(psql "$DB_URL" -tAc "SELECT COUNT(*) FROM route_stops")
vehicles_after=$(psql "$DB_URL" -tAc "SELECT COUNT(*) FROM vehicles")
users_after=$(psql "$DB_URL" -tAc "SELECT COUNT(*) FROM users")
trips_after=$(psql "$DB_URL" -tAc "SELECT COUNT(*) FROM trips")

test "$routes_before" = "$routes_after"
test "$stops_before" = "$stops_after"
test "$route_stops_before" = "$route_stops_after"
test "$vehicles_before" = "$vehicles_after"
test "$users_before" = "$users_after"
test "$trips_before" = "$trips_after"

echo "==> Verify new indexes"

psql "$DB_URL" -c "
SELECT indexname
FROM pg_indexes
WHERE schemaname = 'public'
  AND indexname IN (
    'uq_trips_one_active_per_vehicle',
    'uq_incidents_one_open_per_type',
    'idx_trips_route_status',
    'idx_trips_vehicle_status',
    'idx_incidents_trip_status'
  )
ORDER BY indexname;
"

echo "==> Run schema and constraint tests"

TEST_DATABASE_URL="$DB_URL" go test ./tests/...

echo "==> Migration upgrade test passed"