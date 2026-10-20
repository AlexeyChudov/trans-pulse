#!/bin/bash
# Имитация приложения: раз в секунду пишет запись-пробу. Подключение к ОБОИМ серверам сразу:
# libpq сам выбирает тот, который принимает запись (target_session_attrs=read-write).
cd "$(dirname "$0")/.."; source scripts/lib.sh
URI="postgresql://$APP:$APPLICATION_PASSWORD@127.0.0.1:$PRIMARY,127.0.0.1:$REPLICA/$POSTGRES_DB?sslmode=require&target_session_attrs=read-write&connect_timeout=2"
now() { perl -MTime::HiRes=time -e 'printf "%.3f", time'; }
while true; do
  t=$(now)
  if out=$(psql "$URI" -X -Atq -c "INSERT INTO app.replication_probe (marker) VALUES ('app-' || to_char(clock_timestamp(),'HH24MISSMS') || '-' || pg_backend_pid()) RETURNING id || ' node=' || current_setting('cluster_name')" 2>&1); then
    echo "$t $(date +%H:%M:%S) OK id=$out"
  else
    echo "$t $(date +%H:%M:%S) FAIL $(echo "$out" | head -1 | sed 's/^psql: error: //' | cut -c1-110)"
  fi
  sleep 1
done
