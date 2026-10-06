-- Q2. Какие открытые инциденты заданного типа сейчас есть на маршруте и на каких автобусах?
-- Параметры: тип инцидента, номер маршрута.
\set type 'DELAY'
\set route '1'
SELECT i.id AS incident, i.type, i.created_at, r.number AS route, v.fleet_number AS bus, t.planned_start, t.status AS trip_status
FROM incidents i
JOIN trips t ON t.id = i.trip_id
JOIN routes r ON r.id = t.route_id
JOIN vehicles v ON v.id = t.vehicle_id
WHERE i.status = 'OPEN' AND i.type = :'type' AND r.number = :'route'
ORDER BY i.created_at DESC
LIMIT 10;
