#!/bin/bash
# Аналитик ходит ТОЛЬКО на replica: тяжёлые отчёты не нагружают primary.
cd "$(dirname "$0")/.."; source scripts/lib.sh
echo "Аналитик подключается к replica (порт $REPLICA):"
printf '  это replica (pg_is_in_recovery): '; qa $ANALYST $REPLICA "SELECT pg_is_in_recovery()"
echo "== Отчёт Q1 (доля опозданий по маршрутам) на replica"
psql "$(url $ANALYST $REPLICA)" -X -c '\timing on' -f ../../lab3/sql/queries/q1_route_delays.sql 2>&1 | head -16
echo "== Отчёт Q5 (скорость закрытия инцидентов) на replica"
psql "$(url $ANALYST $REPLICA)" -X -c '\timing on' -f ../../lab3/sql/queries/q5_dispatcher_speed.sql 2>&1 | head -16
echo "== Запись на replica невозможна даже случайно"
try $ANALYST $REPLICA "INSERT INTO routes (number, name) VALUES ('999','x')"
printf '== Сессий аналитика на primary в это время: '; qa $ADMIN $PRIMARY "SELECT count(*) FROM pg_stat_activity WHERE usename='$ANALYST'"
