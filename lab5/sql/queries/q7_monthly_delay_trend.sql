-- Q7. Как меняется доля опозданий от месяца к месяцу (на сколько процентных пунктов хуже или лучше прошлого месяца)?
-- Параметры: период [from, to), порог опоздания в минутах.
-- Приёмы: CTE, агрегат с FILTER, оконная функция LAG() — значение предыдущего месяца, JOIN.
\if :{?from} \else \set from '2025-01-01' \endif
\if :{?to} \else \set to '2026-10-01' \endif
\if :{?threshold} \else \set threshold 5 \endif
WITH monthly AS (
    SELECT date_trunc('month', t.planned_start) AS month,
           count(*) AS trips_done,
           count(*) FILTER (WHERE t.actual_start - t.planned_start > make_interval(mins => :threshold)) AS delayed
    FROM trips t
    JOIN routes r ON r.id = t.route_id
    WHERE t.status = 'COMPLETED' AND r.is_active
      AND t.planned_start >= :'from' AND t.planned_start < :'to'
    GROUP BY 1
), with_pct AS (
    SELECT month, trips_done, delayed, round(100.0 * delayed / trips_done, 1) AS delayed_pct FROM monthly
)
SELECT to_char(month, 'YYYY-MM') AS month, trips_done, delayed, delayed_pct,
       delayed_pct - LAG(delayed_pct) OVER (ORDER BY month) AS change_pp
FROM with_pct
ORDER BY month;
