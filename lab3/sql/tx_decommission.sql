-- Многошаговая транзакция "вывести автобус из эксплуатации" (затрагивает vehicles, trips, incidents).
-- Запуск: psql "$DB" -f lab3/sql/tx_decommission.sql
\set QUIET 1
\pset footer off

-- Автобус в эксплуатации с активным рейсом, открытым инцидентом и плановыми рейсами.
SELECT t.vehicle_id AS v, t.id AS active_trip
FROM trips t
JOIN incidents i ON i.trip_id = t.id AND i.status = 'OPEN'
JOIN vehicles ve ON ve.id = t.vehicle_id AND ve.status = 'IN_SERVICE'
WHERE t.status = 'IN_PROGRESS'
  AND EXISTS (SELECT 1 FROM trips p WHERE p.vehicle_id = t.vehicle_id AND p.status = 'PLANNED')
ORDER BY t.vehicle_id LIMIT 1 \gset
\set user_ok 3
\set user_bad 9999

-- Снимок состояния выбранного автобуса.
\set snapshot 'SELECT (SELECT status FROM vehicles WHERE id = :v) AS vehicle_status, (SELECT count(*) FROM trips WHERE vehicle_id = :v AND status = ''PLANNED'') AS planned_trips, (SELECT status FROM trips WHERE id = :active_trip) AS active_trip_status, (SELECT count(*) FROM incidents WHERE trip_id = :active_trip AND status = ''OPEN'') AS open_incidents'

\echo '=== Исходное состояние (автобус :v, активный рейс :active_trip)'
:snapshot;

\echo '=== Сценарий 1: ROLLBACK. Шаг 4 ссылается на несуществующего пользователя (FK), всё откатывается'
BEGIN;
UPDATE vehicles SET status = 'OUT_OF_SERVICE' WHERE id = :v AND status = 'IN_SERVICE';
UPDATE trips SET status = 'CANCELLED' WHERE vehicle_id = :v AND status = 'PLANNED';
UPDATE trips SET status = 'COMPLETED', actual_end = CURRENT_TIMESTAMP WHERE id = :active_trip AND status = 'IN_PROGRESS';
\echo '--- внутри транзакции (шаги 1-3 выполнены):'
:snapshot;
UPDATE incidents SET status = 'RESOLVED', resolved_at = CURRENT_TIMESTAMP, resolved_by_user_id = :user_bad
 WHERE trip_id = :active_trip AND status = 'OPEN';
ROLLBACK;
\echo '--- после ROLLBACK (ничего не изменилось):'
:snapshot;

\echo '=== Сценарий 2: COMMIT. Те же шаги с существующим пользователем'
BEGIN;
UPDATE vehicles SET status = 'OUT_OF_SERVICE' WHERE id = :v AND status = 'IN_SERVICE';
UPDATE trips SET status = 'CANCELLED' WHERE vehicle_id = :v AND status = 'PLANNED';
UPDATE trips SET status = 'COMPLETED', actual_end = CURRENT_TIMESTAMP WHERE id = :active_trip AND status = 'IN_PROGRESS';
UPDATE incidents SET status = 'RESOLVED', resolved_at = CURRENT_TIMESTAMP, resolved_by_user_id = :user_ok
 WHERE trip_id = :active_trip AND status = 'OPEN';
COMMIT;
\echo '--- после COMMIT:'
:snapshot;
