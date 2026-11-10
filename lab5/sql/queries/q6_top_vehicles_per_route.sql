-- Q6. Какие автобусы чаще всего попадают в инциденты на каждом маршруте (не более top мест в рейтинге)?
-- Параметры: период [from, to), глубина рейтинга top.
-- Приёмы: CTE, оконная функция RANK() OVER (PARTITION BY маршрут ...) — ранжирование внутри группы, JOIN четырёх таблиц.
\if :{?from} \else \set from '2026-01-01' \endif
\if :{?to} \else \set to '2026-10-01' \endif
\if :{?top} \else \set top 3 \endif
WITH per_route_vehicle AS (
    SELECT t.route_id, t.vehicle_id, count(*) AS incidents
    FROM incidents i
    JOIN trips t ON t.id = i.trip_id
    WHERE i.created_at >= :'from' AND i.created_at < :'to'
    GROUP BY t.route_id, t.vehicle_id
), ranked AS (
    SELECT p.route_id, p.vehicle_id, p.incidents,
           RANK() OVER (PARTITION BY p.route_id ORDER BY p.incidents DESC) AS rnk
    FROM per_route_vehicle p
)
SELECT r.number AS route, v.fleet_number AS bus, k.incidents, k.rnk
FROM ranked k
JOIN routes r ON r.id = k.route_id
JOIN vehicles v ON v.id = k.vehicle_id
WHERE k.rnk <= :top
ORDER BY r.id, k.rnk, v.fleet_number
LIMIT 15;
