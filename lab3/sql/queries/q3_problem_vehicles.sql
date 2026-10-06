-- Q3. Какие автобусы чаще всего попадают в инциденты за период (не менее N инцидентов)?
-- Параметры: период [from, to), минимальное число инцидентов.
\set from '2026-09-01'
\set to '2026-10-01'
\set min_incidents 30
SELECT v.fleet_number AS bus, v.model, v.status,
       count(DISTINCT t.id) AS trips_with_incidents,
       count(*) AS incidents
FROM vehicles v
JOIN trips t ON t.vehicle_id = v.id
JOIN incidents i ON i.trip_id = t.id
WHERE i.created_at >= :'from' AND i.created_at < :'to'
GROUP BY v.id, v.fleet_number, v.model, v.status
HAVING count(*) >= :min_incidents
ORDER BY incidents DESC
LIMIT 10;
