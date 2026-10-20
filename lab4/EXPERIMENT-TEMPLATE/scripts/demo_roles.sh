#!/bin/bash
# Роли: что разрешено и запрещено каждой. Запрет суперпользователя для приложения. SSL.
cd "$(dirname "$0")/.."; source scripts/lib.sh
P=$PRIMARY
echo "== 1. Роли кластера и их права (rolsuper = суперпользователь)"
q $ADMIN $P "SELECT rolname, rolsuper, rolcreaterole, rolcreatedb, rolreplication, rolcanlogin FROM pg_roles WHERE rolname IN ('$ADMIN','$APP','$ANALYST','$REPLICATION_USER') ORDER BY rolname"
echo "== 2. Приложение работает под ролью $APP: суперпользователем не является, привилегированных операций выполнить не может"
printf '  %s: rolsuper = ' $APP; qa $APP $P "SELECT rolsuper FROM pg_roles WHERE rolname = current_user"
try $APP $P "CREATE ROLE hacker LOGIN SUPERUSER"
try $APP $P "ALTER SYSTEM SET work_mem = '1GB'"
try $APP $P "COPY routes TO PROGRAM 'id'"
try $APP $P "SELECT pg_read_file('/etc/passwd')"

echo; echo "== 3. $APP (приложение): данные можно, структуру нельзя"
try $APP $P "SELECT count(*) FROM routes"
try $APP $P "INSERT INTO replication_probe (marker) VALUES ('roles-demo-' || clock_timestamp())"
try $APP $P "UPDATE vehicles SET model = model WHERE id = 1"
try $APP $P "DROP TABLE routes"
try $APP $P "TRUNCATE trips"
try $APP $P "ALTER TABLE routes ADD COLUMN x int"
try $APP $P "DELETE FROM incidents WHERE id = 1"
try $APP $P "SELECT * FROM audit_log"

echo; echo "== 4. $ANALYST (аналитик): только чтение, без хэшей паролей"
try $ANALYST $P "SELECT count(*) FROM trips"
try $ANALYST $P "SELECT login, role FROM users LIMIT 1"
try $ANALYST $P "SELECT password_hash FROM users LIMIT 1"
try $ANALYST $P "INSERT INTO routes (number, name) VALUES ('999','x')"
try $ANALYST $P "UPDATE vehicles SET model = 'x'"
try $ANALYST $P "DELETE FROM trips"
try $ANALYST $P "DROP TABLE routes"

echo; echo "== 5. $ADMIN (администратор): управляет структурой, ролями и журналом"
try $ADMIN $P "SELECT count(*) FROM audit_log"
try $ADMIN $P "CREATE TABLE tmp_check (id int)"
try $ADMIN $P "DROP TABLE tmp_check"
try $ADMIN $P "CREATE ROLE tmp_role"
try $ADMIN $P "DROP ROLE tmp_role"

echo; echo "== 6. SSL: небезопасное подключение отклоняется"
printf 'sslmode=disable : '; psql "$(url $APP $P disable)" -X -Atq -c "SELECT 1" 2>&1 | head -1
printf 'sslmode=require : '; psql "$(url $APP $P require)" -X -Atq -c "SELECT 'подключено, SSL=' || ssl FROM pg_stat_ssl WHERE pid = pg_backend_pid()" 2>&1 | head -1
