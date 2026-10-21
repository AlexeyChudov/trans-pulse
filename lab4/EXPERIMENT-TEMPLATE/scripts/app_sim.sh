#!/bin/bash
# Имитация приложения: раз в секунду пишет пробу (sql/02-create-probe.sql). В подключении указаны ОБА сервера:
# libpq сам выбирает тот, который принимает запись (target_session_attrs=read-write).
cd "$(dirname "$0")/.."; source scripts/lib.sh
URI="postgresql://$APP:$APPLICATION_PASSWORD@127.0.0.1:$PRIMARY,127.0.0.1:$REPLICA/$POSTGRES_DB?sslmode=require&target_session_attrs=read-write&connect_timeout=2"
now() { perl -MTime::HiRes=time -e 'printf "%.3f", time'; }
while true; do
  t=$(now)
  if out=$(psql "$URI" -X -Atq -f sql/02-create-probe.sql 2>&1); then
    echo "$t $(date +%H:%M:%S) OK id=$(echo "$out" | grep -m1 '|' | cut -d'|' -f1)"
  else
    echo "$t $(date +%H:%M:%S) FAIL $(echo "$out" | head -1 | sed 's/^psql: error: //' | cut -c1-110)"
  fi
  sleep 1
done
