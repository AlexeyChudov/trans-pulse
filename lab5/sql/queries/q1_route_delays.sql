-- Q1. Какой процент рейсов начинается с опозданием по каждому маршруту за период?
-- Параметры: период [from, to), порог опоздания в минутах. Приёмы: JOIN, GROUP BY, агрегат с FILTER.
\if :{?from} \else \set from '2026-09-01' \endif
\if :{?to} \else \set to '2026-10-01' \endif
\if :{?threshold} \else \set threshold 5 \endif
SELECT r.number AS route,
       count(*) AS trips_done,
       count(*) FILTER (WHERE t.actual_start - t.planned_start > make_interval(mins => :threshold)) AS delayed,
       round(100.0 * count(*) FILTER (WHERE t.actual_start - t.planned_start > make_interval(mins => :threshold)) / count(*), 1) AS delayed_pct
FROM trips t
JOIN routes r ON r.id = t.route_id
WHERE t.status = 'COMPLETED'
  AND t.planned_start >= :'from' AND t.planned_start < :'to'
GROUP BY r.id, r.number
ORDER BY delayed_pct DESC, r.id
LIMIT 10;
