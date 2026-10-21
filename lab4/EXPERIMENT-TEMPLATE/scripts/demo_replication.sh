#!/bin/bash
# Проверка репликации и её направления. Используются готовые SQL-пробы шаблона (sql/01–05).
cd "$(dirname "$0")/.."; source scripts/lib.sh
run() { psql "$(url "$1" "$2")" -X -f "$3"; }   # run <роль> <порт> <файл.sql>

echo "== 1. Статус primary (sql/01-primary-status.sql): is_in_recovery = f, реплика подключена и в состоянии streaming"
run $ADMIN $PRIMARY sql/01-primary-status.sql
echo "== 2. Запись пробы на primary от имени приложения (sql/02-create-probe.sql)"
run $APP $PRIMARY sql/02-create-probe.sql
sleep 1
echo "== 3. Статус replica (sql/03-replica-status.sql): is_in_recovery = t, получение журнала идёт"
run $ADMIN $REPLICA sql/03-replica-status.sql
echo "== 4. Чтение проб на replica (sql/04-read-probes.sql): новая проба на месте"
run $ANALYST $REPLICA sql/04-read-probes.sql
echo "== 5. Запись напрямую на replica запрещена"
psql "$(url $APP $REPLICA)" -X -c "INSERT INTO app.replication_probe (marker) VALUES ('direct-write')" 2>&1
echo; echo "== 6. Целостность на обоих узлах (sql/05-integrity-check.sql)"
echo "primary:"; run $ADMIN $PRIMARY sql/05-integrity-check.sql
echo "replica:"; run $ADMIN $REPLICA sql/05-integrity-check.sql
