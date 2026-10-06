-- Q1. Какой процент рейсов начинается с опозданием по каждому маршруту за период?
-- Параметры: период [from, to), порог опоздания в минутах.
\set from '2026-09-01'
\set to '2026-10-01'
\set threshold 5
SELECT r.number AS route,
       count(*) AS trips_done,
       count(*) FILTER (WHERE t.actual_start - t.planned_start > make_interval(mins => :threshold)) AS delayed,
       round(100.0 * count(*) FILTER (WHERE t.actual_start - t.planned_start > make_interval(mins => :threshold)) / count(*), 1) AS delayed_pct
FROM trips t
JOIN routes r ON r.id = t.route_id
WHERE t.status = 'COMPLETED'
  AND t.planned_start >= :'from' AND t.planned_start < :'to'
GROUP BY r.id, r.number
ORDER BY delayed_pct DESC
LIMIT 10;
