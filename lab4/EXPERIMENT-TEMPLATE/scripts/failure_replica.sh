#!/bin/bash
# Опыт 1: остановка replica. Приложение должно продолжать работать, replica потом догоняет. SQL — пробы шаблона.
cd "$(dirname "$0")/.."; source scripts/lib.sh
run() { psql "$(url "$1" "$2")" -X -f "$3"; }
LOG=results/app_log_replica_down.txt
bash scripts/app_sim.sh > $LOG 2>&1 & APP_PID=$!
trap 'kill $APP_PID 2>/dev/null' EXIT
sleep 4
echo "[$(date +%T)] Останавливаем replica (docker compose stop replica)"
docker compose stop replica >/dev/null 2>&1
sleep 4
echo "== Запись на primary без replica (sql/02-create-probe.sql)"
run $APP $PRIMARY sql/02-create-probe.sql
echo "== primary видит реплику? (sql/01-primary-status.sql): вторая таблица должна быть пустой"
run $ADMIN $PRIMARY sql/01-primary-status.sql
sleep 3
echo "[$(date +%T)] Запускаем replica обратно"
docker compose start replica >/dev/null 2>&1
wait_ready $REPLICA; sleep 4
kill $APP_PID 2>/dev/null; wait $APP_PID 2>/dev/null
echo "== Replica догнала? (sql/03-replica-status.sql, sql/04-read-probes.sql)"
run $ADMIN $REPLICA sql/03-replica-status.sql
run $ANALYST $REPLICA sql/04-read-probes.sql | head -8
echo "== Целостность (sql/05-integrity-check.sql): число проб должно совпасть"
echo "primary:"; run $ADMIN $PRIMARY sql/05-integrity-check.sql | sed -n 1,5p
echo "replica:"; run $ADMIN $REPLICA sql/05-integrity-check.sql | sed -n 1,5p
echo; echo "== Поведение приложения (журнал $LOG): записей OK=$(grep -c ' OK ' $LOG), FAIL=$(grep -c ' FAIL ' $LOG)"
