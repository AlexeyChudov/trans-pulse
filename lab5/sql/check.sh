#!/bin/bash
# Автоматическая проверка SQL-запросов: тестовая БД + фикстура + сравнение с эталонными результатами (expected/).
#   bash lab5/sql/check.sh            — проверить все запросы
#   bash lab5/sql/check.sh --update   — перезаписать эталоны текущим выводом (только после ручной проверки!)
# Нужен запущенный PostgreSQL (по умолчанию localhost:5432) и goose.
set -u
cd "$(dirname "$0")"
PG=${CHECK_PG:-postgres://postgres:postgres@localhost:5432}
DB=trans-pulse_lab5_test
UPDATE=0; [ "${1:-}" = "--update" ] && UPDATE=1
export PGOPTIONS='-c timezone=UTC'   # даты и время в результатах не зависят от часового пояса машины

psql "$PG/postgres?sslmode=disable" -Atqc "SELECT 1 FROM pg_database WHERE datname='$DB'" | grep -q 1 \
  || psql "$PG/postgres?sslmode=disable" -qc "CREATE DATABASE \"$DB\"" || exit 2
URL="$PG/$DB?sslmode=disable"
psql "$URL" -qc "SET client_min_messages = warning; DROP SCHEMA public CASCADE; CREATE SCHEMA public;" || exit 2
goose -dir ../../migrations postgres "$URL" up >/dev/null || exit 2
psql "$URL" -q -v ON_ERROR_STOP=1 -f fixture.sql || exit 2

# параметры запросов для маленького набора данных
params() { case $1 in
  1) echo "-v from=2026-09-01 -v to=2026-10-01 -v threshold=5";;
  2) echo "-v type=DELAY -v route=1";;
  3) echo "-v from=2026-09-01 -v to=2026-11-01 -v min_incidents=3";;
  4) echo "-v min_routes=2";;
  5) echo "-v from=2026-09-01 -v to=2026-11-01";;
  6) echo "-v from=2026-09-01 -v to=2026-11-01 -v top=3";;
  7) echo "-v from=2026-09-01 -v to=2026-11-01 -v threshold=5";;
  8) echo "-v from=2026-09-01 -v to=2026-09-08";;
esac; }
FAIL=0
for n in 1 2 3 4 5 6 7 8; do
  file=$(ls queries/q${n}_*.sql)
  actual=$(psql "$URL" -X -A -F'|' -P footer=off $(params $n) -f "$file" 2>&1)
  if [ $UPDATE = 1 ]; then echo "$actual" > expected/q$n.txt; echo "обновлён expected/q$n.txt"; continue; fi
  if [ "$actual" = "$(cat expected/q$n.txt)" ]; then printf '  ✓ Q%s  %s\n' $n "$(basename "$file" .sql)"
  else printf '  ✗ Q%s  %s — результат не совпал с эталоном:\n' $n "$(basename "$file" .sql)"; diff <(cat expected/q$n.txt) <(echo "$actual") | sed 's/^/      /'; FAIL=1; fi
done
[ $UPDATE = 1 ] && exit 0
[ $FAIL = 0 ] && echo "ВСЕ ПРОВЕРКИ ПРОЙДЕНЫ" || { echo "ЕСТЬ РАСХОЖДЕНИЯ"; exit 1; }
