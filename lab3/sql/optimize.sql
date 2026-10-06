-- Индексы для оптимизации Q1 и Q2 (применяются отдельно от миграций).
-- Q1: фильтр по status + диапазону planned_start, нужные столбцы в INCLUDE -> Index Only Scan.
CREATE INDEX idx_trips_status_start_cov
    ON trips (status, planned_start) INCLUDE (route_id, actual_start);
-- Q2: открытые инциденты (~0,5% таблицы), фильтр по типу, сортировка по created_at.
CREATE INDEX idx_incidents_open_type_created
    ON incidents (type, created_at DESC) WHERE status = 'OPEN';
