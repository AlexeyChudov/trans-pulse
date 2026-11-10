-- Маленький набор данных для автоматической проверки SQL-запросов. Результаты запросов на нём вычислены вручную
-- (см. expected/). Загружается в пустую БД с миграциями: идентификаторы идут подряд с 1.
INSERT INTO routes (number, name) VALUES ('1', 'Центр — Вокзал'), ('2', 'Университет — Парк');

INSERT INTO stops (name, latitude, longitude) VALUES
  ('Центральная', 50.4501, 30.5234), ('Вокзал', 50.4419, 30.5118), ('Университет', 50.4489, 30.5145), ('Парк', 50.4550, 30.5200);

-- маршрут 1: Центральная, Вокзал, Университет; маршрут 2: Университет, Парк, Центральная
INSERT INTO route_stops (route_id, stop_id, sequence_no) VALUES (1,1,1), (1,2,2), (1,3,3), (2,3,1), (2,4,2), (2,1,3);

INSERT INTO vehicles (fleet_number, model, status) VALUES ('B1', 'MAN', 'IN_SERVICE'), ('B2', 'Volvo', 'IN_SERVICE'), ('B3', 'Citaro', 'OUT_OF_SERVICE');

INSERT INTO users (login, password_hash, role) VALUES ('adm', 'x', 'ADMIN'), ('disp1', 'x', 'DISPATCHER'), ('disp2', 'x', 'DISPATCHER');

-- Рейсы (маршрут, автобус, плановый старт, опоздание в минутах). Все по 30 минут, статус COMPLETED, кроме T10.
INSERT INTO trips (route_id, vehicle_id, planned_start, planned_end, actual_start, actual_end, status)
SELECT route_id, vehicle_id, ps, ps + interval '30 minutes', ps + delay * interval '1 minute', ps + delay * interval '1 minute' + interval '30 minutes', 'COMPLETED'
FROM (VALUES
  (1, 1, timestamptz '2026-09-01 08:00+00',  0),   -- T1
  (1, 1, timestamptz '2026-09-02 08:00+00', 10),   -- T2
  (1, 2, timestamptz '2026-09-03 08:00+00',  3),   -- T3
  (2, 2, timestamptz '2026-09-04 08:00+00',  7),   -- T4
  (2, 1, timestamptz '2026-09-05 08:00+00',  0),   -- T5
  (1, 1, timestamptz '2026-10-01 08:00+00',  8),   -- T6
  (1, 2, timestamptz '2026-10-02 08:00+00', 20),   -- T7
  (2, 2, timestamptz '2026-10-03 08:00+00',  1),   -- T8
  (1, 1, timestamptz '2026-11-01 08:00+00',  6)    -- T9 (вне периодов запросов)
) AS x(route_id, vehicle_id, ps, delay);
-- T10: отменённый рейс (в расчётах опозданий не участвует)
INSERT INTO trips (route_id, vehicle_id, planned_start, planned_end, status)
VALUES (1, 3, '2026-09-06 08:00+00', '2026-09-06 08:30+00', 'CANCELLED');

-- Инциденты I1..I9 (id по порядку): рейс, тип, создан, закрыт, кем
INSERT INTO incidents (trip_id, type, status, created_at, resolved_at, resolved_by_user_id) VALUES
  (2, 'DELAY',        'RESOLVED', '2026-09-02 08:10+00', '2026-09-02 08:20+00', 2),  -- I1, 10 мин
  (2, 'NO_TELEMETRY', 'RESOLVED', '2026-09-02 08:15+00', '2026-09-02 08:45+00', 2),  -- I2, 30 мин
  (3, 'DELAY',        'RESOLVED', '2026-09-03 08:05+00', '2026-09-03 08:25+00', 3),  -- I3, 20 мин
  (4, 'DELAY',        'RESOLVED', '2026-09-04 08:10+00', '2026-09-04 08:40+00', 3),  -- I4, 30 мин
  (6, 'DELAY',        'RESOLVED', '2026-10-01 08:10+00', '2026-10-01 08:30+00', 2),  -- I5, 20 мин
  (7, 'DELAY',        'OPEN',     '2026-10-02 08:10+00', NULL, NULL),                -- I6
  (7, 'NO_TELEMETRY', 'OPEN',     '2026-10-02 08:20+00', NULL, NULL),                -- I7
  (8, 'DELAY',        'RESOLVED', '2026-10-03 08:10+00', '2026-10-03 08:20+00', 2),  -- I8, 10 мин
  (5, 'DELAY',        'RESOLVED', '2026-09-05 08:05+00', '2026-09-05 08:15+00', 2);  -- I9, 10 мин
