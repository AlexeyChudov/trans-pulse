\echo '== Размеры таблиц'
SELECT relname AS table, reltuples::bigint AS rows, pg_size_pretty(pg_total_relation_size(oid)) AS total_size
FROM pg_class WHERE relname IN ('routes','stops','route_stops','vehicles','users','trips','incidents') ORDER BY pg_total_relation_size(oid) DESC;
\echo '== Статусы рейсов'
SELECT status, count(*), round(100.0*count(*)/sum(count(*)) OVER (),2) AS pct FROM trips GROUP BY status ORDER BY 2 DESC;
\echo '== Статусы автобусов'
SELECT status, count(*) FROM vehicles GROUP BY status ORDER BY 2 DESC;
\echo '== Популярные и редкие маршруты (доля рейсов)'
(SELECT route_id, count(*), round(100.0*count(*)/3000000,2) AS pct FROM trips GROUP BY 1 ORDER BY 2 DESC LIMIT 3)
UNION ALL
(SELECT route_id, count(*), round(100.0*count(*)/3000000,2) FROM trips GROUP BY 1 ORDER BY 2 ASC LIMIT 3) ORDER BY 2 DESC;
\echo '== Популярные и редкие автобусы'
(SELECT vehicle_id, count(*), round(100.0*count(*)/3000000,3) AS pct FROM trips GROUP BY 1 ORDER BY 2 DESC LIMIT 3)
UNION ALL
(SELECT vehicle_id, count(*), round(100.0*count(*)/3000000,3) FROM trips GROUP BY 1 ORDER BY 2 ASC LIMIT 3) ORDER BY 2 DESC;
\echo '== Рейсы по годам-месяцам (первые/последние)'
SELECT to_char(date_trunc('quarter', planned_start),'YYYY-"Q"Q') AS quarter, count(*) FROM trips GROUP BY 1 ORDER BY 1;
\echo '== Рейсы по часам суток (UTC)'
SELECT extract(hour FROM planned_start)::int AS hour, count(*) FROM trips GROUP BY 1 ORDER BY 1;
\echo '== Инциденты: тип и статус'
SELECT type, status, count(*) FROM incidents GROUP BY 1,2 ORDER BY 1,2;
\echo '== Остановки: в скольких маршрутах состоят'
SELECT routes_cnt, count(*) AS stops FROM (SELECT s.id, count(rs.route_id) AS routes_cnt FROM stops s LEFT JOIN route_stops rs ON rs.stop_id=s.id GROUP BY s.id) x GROUP BY 1 ORDER BY 1;
\echo '== Проверки целостности (ожидается 0)'
SELECT count(*) AS vehicles_with_2_in_progress FROM (SELECT vehicle_id FROM trips WHERE status='IN_PROGRESS' GROUP BY 1 HAVING count(*)>1) x;
SELECT count(*) AS bad_planned FROM trips WHERE planned_end <= planned_start;
