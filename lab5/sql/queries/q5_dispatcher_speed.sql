-- Q5. Как быстро диспетчеры закрывают инциденты каждого типа за период?
-- Параметры: период закрытия [from, to). Приёмы: JOIN, GROUP BY, среднее и медиана (percentile_cont).
\if :{?from} \else \set from '2026-01-01' \endif
\if :{?to} \else \set to '2026-10-01' \endif
SELECT u.login, i.type,
       count(*) AS resolved,
       round(avg(extract(epoch FROM i.resolved_at - i.created_at) / 60)::numeric, 1) AS avg_min,
       round((percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM i.resolved_at - i.created_at) / 60))::numeric, 1) AS median_min
FROM incidents i
JOIN users u ON u.id = i.resolved_by_user_id
WHERE i.status = 'RESOLVED' AND i.resolved_at >= :'from' AND i.resolved_at < :'to'
GROUP BY u.id, u.login, i.type
ORDER BY resolved DESC, u.login, i.type
LIMIT 10;
