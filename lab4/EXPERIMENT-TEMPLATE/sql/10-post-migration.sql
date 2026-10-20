-- Выполняется от администратора ПОСЛЕ миграций (таблицы проекта лежат в схеме app).
-- Права на таблицы приложению и аналитику уже выданы шаблоном (ALTER DEFAULT PRIVILEGES в 01-initialize-cluster.sh).
SET search_path = app;

-- 1. Аналитик не видит хэши паролей: вместо права на всю таблицу — права на столбцы
REVOKE SELECT ON users FROM course_analyst;
GRANT SELECT (id, login, role, is_active, created_at) ON users TO course_analyst;

-- 2. Инциденты не удаляются (правило предметной области)
REVOKE DELETE ON incidents FROM course_app;

-- 3. RLS на incidents: диспетчер закрывает инцидент только от своего имени.
--    Приложение перед работой сообщает, кто пользователь:  SET app.user_id = '3';
--    Администратор (суперпользователь) политиками не ограничивается.
ALTER TABLE incidents ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS incidents_read ON incidents;
DROP POLICY IF EXISTS incidents_create ON incidents;
DROP POLICY IF EXISTS incidents_resolve ON incidents;
CREATE POLICY incidents_read ON incidents FOR SELECT TO course_app, course_analyst USING (true);
CREATE POLICY incidents_create ON incidents FOR INSERT TO course_app WITH CHECK (status = 'OPEN');
CREATE POLICY incidents_resolve ON incidents FOR UPDATE TO course_app
    USING (status = 'OPEN')
    WITH CHECK (status = 'RESOLVED'
                AND resolved_by_user_id = nullif(current_setting('app.user_id', true), '')::bigint);

-- 4. Аудит критических операций: смена статуса автобуса, отмена рейса, закрытие инцидента
CREATE TABLE IF NOT EXISTS audit_log (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    db_user TEXT NOT NULL DEFAULT session_user,
    app_user TEXT,
    table_name TEXT NOT NULL,
    row_id BIGINT NOT NULL,
    action TEXT NOT NULL,
    old_value TEXT,
    new_value TEXT
);
REVOKE ALL ON audit_log FROM course_app, course_analyst;  -- права по умолчанию выдали бы их автоматически
-- Запись идёт через функцию владельца (SECURITY DEFINER): приложение само журнал изменить не может
CREATE OR REPLACE FUNCTION audit_critical() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = app AS $$
BEGIN
    IF TG_TABLE_NAME = 'vehicles' AND NEW.status IS DISTINCT FROM OLD.status THEN
        INSERT INTO audit_log (app_user, table_name, row_id, action, old_value, new_value)
        VALUES (current_setting('app.user_id', true), 'vehicles', NEW.id, 'STATUS_CHANGE', OLD.status, NEW.status);
    ELSIF TG_TABLE_NAME = 'trips' AND NEW.status = 'CANCELLED' AND OLD.status IS DISTINCT FROM 'CANCELLED' THEN
        INSERT INTO audit_log (app_user, table_name, row_id, action, old_value, new_value)
        VALUES (current_setting('app.user_id', true), 'trips', NEW.id, 'TRIP_CANCELLED', OLD.status, NEW.status);
    ELSIF TG_TABLE_NAME = 'incidents' AND NEW.status = 'RESOLVED' AND OLD.status = 'OPEN' THEN
        INSERT INTO audit_log (app_user, table_name, row_id, action, old_value, new_value)
        VALUES (current_setting('app.user_id', true), 'incidents', NEW.id, 'INCIDENT_RESOLVED', OLD.status, NEW.status);
    END IF;
    RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS trg_audit_vehicles ON vehicles;
DROP TRIGGER IF EXISTS trg_audit_trips ON trips;
DROP TRIGGER IF EXISTS trg_audit_incidents ON incidents;
CREATE TRIGGER trg_audit_vehicles AFTER UPDATE ON vehicles FOR EACH ROW EXECUTE FUNCTION audit_critical();
CREATE TRIGGER trg_audit_trips AFTER UPDATE ON trips FOR EACH ROW EXECUTE FUNCTION audit_critical();
CREATE TRIGGER trg_audit_incidents AFTER UPDATE ON incidents FOR EACH ROW EXECUTE FUNCTION audit_critical();
