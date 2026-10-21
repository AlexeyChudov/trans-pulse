#!/bin/bash
# Опыт 2: падение primary. Фиксируем простой приложения, повышаем replica до primary, проверяем данные,
# возвращаем старый primary как replica. SQL — пробы шаблона (sql/01–05).
cd "$(dirname "$0")/.."; source scripts/lib.sh
run() { psql "$(url "$1" "$2")" -X -f "$3"; }
LOG=results/app_log_primary_down.txt
bash scripts/app_sim.sh > $LOG 2>&1 & APP_PID=$!
trap 'kill $APP_PID 2>/dev/null' EXIT
sleep 4
echo "[$(date +%T)] Эмулируем аварию: docker compose kill primary"
docker compose kill primary >/dev/null 2>&1
sleep 5
echo "== Диагностика: primary недоступен?"; pg_isready -h 127.0.0.1 -p $PRIMARY
LASTOK=$(grep ' OK ' $LOG | tail -1 | sed 's/.*id=\([0-9]*\).*/\1/')
echo "== Диагностика: состояние replica (sql/03-replica-status.sql): таблица wal receiver пуста — связи с primary нет"
run $ADMIN $REPLICA sql/03-replica-status.sql
echo "[$(date +%T)] ВОССТАНОВЛЕНИЕ: повышаем replica до primary (pg_promote)"
docker compose exec -T replica psql -U $ADMIN -d $POSTGRES_DB -Atc "SELECT pg_promote(wait => true)"
sleep 4
kill $APP_PID 2>/dev/null; wait $APP_PID 2>/dev/null
echo; echo "== Хронология приложения (журнал $LOG), только смены состояния:"
awk '{st=$3} st!=prev {print; prev=st}' $LOG
FIRSTFAIL=$(grep ' FAIL ' $LOG | head -1 | awk '{print $1}')
RECOVER=$(awk -v f="$FIRSTFAIL" '$1>f && $3=="OK" {print $1; exit}' $LOG)
printf 'Простой приложения (первая ошибка -> первая успешная запись): %.1f с (из них ~5 с мы сами «замечали» аварию до promote)\n' "$(echo "$RECOVER - $FIRSTFAIL" | bc)"
MAXID=$(psql "$(url $ADMIN $REPLICA)" -X -Atq -c "SELECT max(id) FROM app.replication_probe")
echo "Последняя запись, подтверждённая приложением на старом primary: id=$LASTOK; максимум на новом primary: id=$MAXID; потеряно записей: $((LASTOK-MAXID > 0 ? LASTOK-MAXID : 0))"

echo; echo "== Новый primary: статус (sql/01-primary-status.sql): is_in_recovery = f"
run $ADMIN $REPLICA sql/01-primary-status.sql
echo "== Новый primary принимает запись от приложения (sql/02-create-probe.sql)"
run $APP $REPLICA sql/02-create-probe.sql
echo "== Целостность на новом primary (sql/05-integrity-check.sql)"
run $ADMIN $REPLICA sql/05-integrity-check.sql

echo; echo "== Возвращаем старый primary как replica нового (свежая базовая копия)"
docker compose rm -sf primary >/dev/null 2>&1
docker volume rm sprint04_primary_data >/dev/null
docker run --rm -v sprint04_primary_data:/var/lib/postgresql/data --network sprint04_database_network \
  -e PGPASSWORD="$REPLICATION_PASSWORD" postgres:17-bookworm bash -c \
  "chown postgres:postgres /var/lib/postgresql/data && chmod 700 /var/lib/postgresql/data && \
   gosu postgres pg_basebackup -h replica -U $REPLICATION_USER -D /var/lib/postgresql/data -Fp -Xs -R -c fast -C -S slot_old_primary" 2>&1 | tail -2
docker compose up -d primary >/dev/null 2>&1
wait_ready $PRIMARY
echo "== Старый primary (порт $PRIMARY) теперь replica (sql/03-replica-status.sql) и получает журнал"
run $ADMIN $PRIMARY sql/03-replica-status.sql
echo "== Новый primary видит его в репликации (sql/01-primary-status.sql)"
run $ADMIN $REPLICA sql/01-primary-status.sql
echo "Роли поменялись: primary = порт $REPLICA, replica = порт $PRIMARY. Вернуть исходную схему: scripts/reset.sh"
