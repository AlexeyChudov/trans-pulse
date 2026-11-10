-- Q2. Какие открытые инциденты заданного типа есть на маршруте и на каких автобусах?
-- Параметры: тип инцидента, номер маршрута. Приёмы: JOIN четырёх таблиц.
\if :{?type} \else \set type 'DELAY' \endif
\if :{?route} \else \set route '1' \endif
SELECT i.id AS incident, i.type, i.created_at, r.number AS route, v.fleet_number AS bus, t.planned_start, t.status AS trip_status
FROM incidents i
JOIN trips t ON t.id = i.trip_id
JOIN routes r ON r.id = t.route_id
JOIN vehicles v ON v.id = t.vehicle_id
WHERE i.status = 'OPEN' AND i.type = :'type' AND r.number = :'route'
ORDER BY i.created_at DESC, i.id
LIMIT 10;
