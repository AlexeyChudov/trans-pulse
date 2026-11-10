-- Q8. Сколько инцидентов закрывают диспетчеры по дням: накопительно и в среднем за последние 7 дней?
-- Параметры: период [from, to) (по календарным дням, дни без закрытий считаются нулями).
-- Приёмы: CTE, календарь generate_series + LEFT JOIN, SUM() OVER (ORDER BY день) — накопительный итог,
-- AVG() OVER (ROWS BETWEEN 6 PRECEDING AND CURRENT ROW) — скользящее среднее, JOIN четырёх таблиц.
\if :{?from} \else \set from '2026-09-01' \endif
\if :{?to} \else \set to '2026-10-01' \endif
WITH calendar AS (
    SELECT d::date AS day FROM generate_series(:'from'::date, :'to'::date - 1, interval '1 day') AS d
), daily AS (
    SELECT i.resolved_at::date AS day, count(*) AS resolved
    FROM incidents i
    JOIN users u ON u.id = i.resolved_by_user_id AND u.role = 'DISPATCHER'
    JOIN trips t ON t.id = i.trip_id
    JOIN routes r ON r.id = t.route_id AND r.is_active
    WHERE i.status = 'RESOLVED'
      AND i.resolved_at >= :'from' AND i.resolved_at < :'to'
    GROUP BY 1
)
SELECT c.day,
       coalesce(d.resolved, 0) AS resolved,
       SUM(coalesce(d.resolved, 0)) OVER (ORDER BY c.day) AS cumulative,
       round(AVG(coalesce(d.resolved, 0)) OVER (ORDER BY c.day ROWS BETWEEN 6 PRECEDING AND CURRENT ROW), 1) AS avg_7d
FROM calendar c
LEFT JOIN daily d ON d.day = c.day
ORDER BY c.day;
