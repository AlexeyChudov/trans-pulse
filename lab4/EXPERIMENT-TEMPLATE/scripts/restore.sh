#!/bin/bash
# ВОССТАНОВЛЕНИЕ в ЧИСТЫЙ узел restore (сервис шаблона, порт 55434) и проверка. Нужна копия от scripts/backup.sh.
cd "$(dirname "$0")/.."; source scripts/lib.sh
[ -f backups/course_app.dump ] || { echo "Нет backups/course_app.dump — сначала scripts/backup.sh"; exit 1; }
RESTORE_PORT=${RESTORE_PORT:-55434}
RURL="postgresql://$ADMIN:$RESTORE_ADMIN_PASSWORD@127.0.0.1:$RESTORE_PORT/$RESTORE_DB?sslmode=prefer"
RPG="postgresql://$ADMIN:$RESTORE_ADMIN_PASSWORD@127.0.0.1:$RESTORE_PORT/postgres?sslmode=prefer"

echo "== 1. Чистый узел restore"
docker compose --profile restore rm -sfv restore >/dev/null 2>&1; docker volume rm -f sprint04_restore_data >/dev/null
docker compose --profile restore up -d restore >/dev/null 2>&1
until psql "$RURL" -X -Atq -c "SELECT 1" >/dev/null 2>&1; do sleep 1; done
printf 'таблиц до восстановления: '; psql "$RURL" -X -Atq -c "SELECT count(*) FROM pg_tables WHERE schemaname NOT IN ('pg_catalog','information_schema')"

echo "== 2. Восстановление: сначала роли, потом данные"
grep -v -E "ROLE $ADMIN\b" backups/globals.sql | psql "$RPG" -X -q 2>&1 | grep -v "already exists"
pg_restore --dbname="$RURL" backups/course_app.dump && echo "pg_restore завершён без ошибок"

echo "== 3. Проверка: SQL-проба шаблона sql/05-integrity-check.sql на оригинале и на копии"
psql "$(url $ADMIN $PRIMARY)" -X -f sql/05-integrity-check.sql > /tmp/check_orig.txt
psql "$RURL" -X -f sql/05-integrity-check.sql > /tmp/check_copy.txt
cat /tmp/check_copy.txt
# имена ограничений NOT NULL содержат внутренние номера объектов (у каждой БД свои) — при сравнении их вырезаем
norm() { sed -E 's/[0-9]+_[0-9]+_([0-9]+_not_null)/N_N_\1/' "$1"; }
diff -q <(norm /tmp/check_orig.txt) <(norm /tmp/check_copy.txt) >/dev/null && echo "Проба: результаты на оригинале и копии ПОЛНОСТЬЮ совпадают" || echo "Проба: РАЗЛИЧИЯ"
for t in routes vehicles trips incidents; do
  printf '%-10s оригинал=%-7s копия=%s\n' $t "$(psql "$(url $ADMIN $PRIMARY)" -X -Atq -c "SELECT count(*) FROM app.$t")" "$(psql "$RURL" -X -Atq -c "SELECT count(*) FROM app.$t")"
done
A=$(psql "$(url $ADMIN $PRIMARY)" -X -At -f ../../lab3/sql/checksum.sql)
B=$(psql "$RURL" -X -At -c "SET search_path=app" -f ../../lab3/sql/checksum.sql | tail -1)
[ "$A" = "$B" ] && echo "Контрольная сумма данных СОВПАДАЕТ — восстановлено полностью" || echo "Контрольная сумма НЕ совпадает"
