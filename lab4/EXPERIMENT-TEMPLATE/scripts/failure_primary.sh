#!/bin/bash
# Опыт 2: падение primary. Фиксируем простой приложения, переводим replica в primary, проверяем данные,
# затем возвращаем старый primary в строй как replica нового.
cd "$(dirname "$0")/.."; source scripts/lib.sh
LOG=results/app_log_primary_down.txt
bash scripts/app_sim.sh > $LOG 2>&1 & APP_PID=$!
trap 'kill $APP_PID 2>/dev/null' EXIT
sleep 4
BEFORE=$(qa $ADMIN $PRIMARY "SELECT count(*) FROM trips")
echo "[$(date +%T)] Эмулируем аварию: docker compose kill primary"
docker compose kill primary >/dev/null 2>&1
sleep 5
echo "[$(date +%T)] ДИАГНОСТИКА: primary недоступен?"
pg_isready -h 127.0.0.1 -p $PRIMARY
echo "[$(date +%T)] ДИАГНОСТИКА: что с replica (docker compose exec, локально внутри контейнера)"
docker compose exec -T replica psql -U $ADMIN -d $POSTGRES_DB -Atc "SELECT 'pg_is_in_recovery=' || pg_is_in_recovery()"
docker compose exec -T replica psql -U $ADMIN -d $POSTGRES_DB -Atc "SELECT 'связь с primary (pg_stat_wal_receiver): ' || coalesce((SELECT status FROM pg_stat_wal_receiver), 'нет соединения')"
LASTOK=$(grep ' OK ' $LOG | tail -1 | sed 's/.*id=\([0-9]*\) .*/\1/')
echo "[$(date +%T)] ВОССТАНОВЛЕНИЕ: переводим replica в primary (pg_promote)"
docker compose exec -T replica psql -U $ADMIN -d $POSTGRES_DB -Atc "SELECT pg_promote(wait => true)"
sleep 4
kill $APP_PID 2>/dev/null; wait $APP_PID 2>/dev/null
echo; echo "== Хронология приложения (журнал $LOG), только смены состояния:"
awk '{st=$3} st!=prev {print; prev=st}' $LOG
FIRSTFAIL=$(grep ' FAIL ' $LOG | head -1 | awk '{print $1}')
RECOVER=$(awk -v f="$FIRSTFAIL" '$1>f && $3=="OK" {print $1; exit}' $LOG)
printf 'Простой приложения (первая ошибка -> первая успешная запись): %.1f с (из них ~5 с мы сами «замечали» аварию до promote)\n' "$(echo "$RECOVER - $FIRSTFAIL" | bc)"
echo; echo "== Проверка целостности на новом primary (бывшая replica, порт $REPLICA)"
printf 'pg_is_in_recovery: '; qa $ADMIN $REPLICA "SELECT pg_is_in_recovery()"
AFTER=$(qa $ADMIN $REPLICA "SELECT count(*) FROM trips")
echo "рейсов до аварии: $BEFORE, после: $AFTER $([ "$BEFORE" = "$AFTER" ] && echo '(совпадает)')"
MAXID=$(qa $ADMIN $REPLICA "SELECT max(id) FROM app.replication_probe")
echo "последняя запись, подтверждённая приложением на старом primary: id=$LASTOK; максимум на новом primary: id=$MAXID; потеряно записей: $((LASTOK-MAXID > 0 ? LASTOK-MAXID : 0))"
printf 'рейсов без маршрута или автобуса (ожидается 0): '; qa $ADMIN $REPLICA "SELECT count(*) FROM trips t WHERE NOT EXISTS (SELECT 1 FROM routes r WHERE r.id=t.route_id) OR NOT EXISTS (SELECT 1 FROM vehicles v WHERE v.id=t.vehicle_id)"
printf 'автобусов с двумя активными рейсами (ожидается 0): '; qa $ADMIN $REPLICA "SELECT count(*) FROM (SELECT vehicle_id FROM trips WHERE status='IN_PROGRESS' GROUP BY 1 HAVING count(*)>1) x"
printf 'запись от имени приложения на новый primary: '; qa $APP $REPLICA "INSERT INTO app.replication_probe (marker) VALUES ('after-failover-' || clock_timestamp()) RETURNING 'ok id=' || id"
psql "$(url $ADMIN $REPLICA)" -X -f sql/05-integrity-check.sql 2>&1 | head -8

echo; echo "== Возвращаем старый primary в строй как replica нового (свежая базовая копия с нового primary)"
docker compose rm -sf primary >/dev/null 2>&1
docker volume rm sprint04_primary_data >/dev/null
docker run --rm -v sprint04_primary_data:/var/lib/postgresql/data --network sprint04_database_network \
  -e PGPASSWORD="$REPLICATION_PASSWORD" postgres:17-bookworm bash -c \
  "chown postgres:postgres /var/lib/postgresql/data && chmod 700 /var/lib/postgresql/data && \
   gosu postgres pg_basebackup -h replica -U $REPLICATION_USER -D /var/lib/postgresql/data -Fp -Xs -R -c fast -C -S slot_old_primary" 2>&1 | tail -2
docker compose up -d primary >/dev/null 2>&1
wait_ready $PRIMARY
printf 'старый primary теперь replica (pg_is_in_recovery): '; qa $ADMIN $PRIMARY "SELECT pg_is_in_recovery()"
q $ADMIN $REPLICA "SELECT application_name, state, replay_lag FROM pg_stat_replication"
ID=$(qa $APP $REPLICA "INSERT INTO app.replication_probe (marker) VALUES ('after-rebuild-' || clock_timestamp()) RETURNING id"); sleep 1
printf 'новая запись id=%s на новом primary видна на пересобранном узле: ' "$ID"; qa $ADMIN $PRIMARY "SELECT count(*) FROM app.replication_probe WHERE id=$ID"
echo "Роли поменялись: primary = порт $REPLICA, replica = порт $PRIMARY. Для возврата к исходной схеме — scripts/reset.sh"
