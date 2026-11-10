-- Q4. Какие остановки — пересадочные узлы (входят не менее чем в N активных маршрутов)?
-- Параметры: минимальное число маршрутов. Приёмы: JOIN трёх таблиц, GROUP BY, HAVING, string_agg.
\if :{?min_routes} \else \set min_routes 15 \endif
SELECT s.id AS stop, s.name,
       count(DISTINCT r.id) AS routes,
       string_agg(DISTINCT r.number, ',' ORDER BY r.number) AS route_numbers
FROM stops s
JOIN route_stops rs ON rs.stop_id = s.id
JOIN routes r ON r.id = rs.route_id
WHERE r.is_active AND s.is_active
GROUP BY s.id, s.name
HAVING count(DISTINCT r.id) >= :min_routes
ORDER BY routes DESC, s.id
LIMIT 10;
