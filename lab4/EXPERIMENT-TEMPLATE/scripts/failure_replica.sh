#!/bin/bash
# Опыт 1: остановка replica. Приложение должно продолжать работать, replica потом догоняет.
cd "$(dirname "$0")/.."; source scripts/lib.sh
LOG=results/app_log_replica_down.txt
bash scripts/app_sim.sh > $LOG 2>&1 & APP_PID=$!
trap 'kill $APP_PID 2>/dev/null' EXIT
sleep 4
echo "[$(date +%T)] Останавливаем replica (docker compose stop replica)"
docker compose stop replica >/dev/null 2>&1
sleep 5
echo "[$(date +%T)] primary видит реплику?"
q $ADMIN $PRIMARY "SELECT count(*) AS подключённых_реплик FROM pg_stat_replication"
q $ADMIN $PRIMARY "SELECT slot_name, active FROM pg_replication_slots"
printf 'записей-проб на primary за время простоя replica продолжает расти: '; qa $ADMIN $PRIMARY "SELECT count(*) FROM app.replication_probe"
sleep 3
echo "[$(date +%T)] Запускаем replica обратно"
docker compose start replica >/dev/null 2>&1
wait_ready $REPLICA; sleep 4
kill $APP_PID 2>/dev/null; wait $APP_PID 2>/dev/null
echo "[$(date +%T)] Replica догнала primary?"
q $ADMIN $PRIMARY "SELECT application_name, state, sync_state, replay_lag FROM pg_stat_replication"
printf 'строк replication_probe: primary=%s replica=%s\n' "$(qa $ADMIN $PRIMARY 'SELECT count(*) FROM app.replication_probe')" "$(qa $ADMIN $REPLICA 'SELECT count(*) FROM app.replication_probe')"
echo; echo "== Поведение приложения (журнал $LOG): записей OK=$(grep -c ' OK ' $LOG), FAIL=$(grep -c ' FAIL ' $LOG)"
