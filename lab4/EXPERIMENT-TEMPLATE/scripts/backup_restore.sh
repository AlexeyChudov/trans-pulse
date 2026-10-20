#!/bin/bash
# Резервная копия с primary и восстановление в ЧИСТЫЙ экземпляр (сервис restore из шаблона, порт 55434).
cd "$(dirname "$0")/.."; source scripts/lib.sh
mkdir -p backups
TABLES="routes stops route_stops vehicles users trips incidents replication_probe audit_log"
RURL="postgresql://$ADMIN:$RESTORE_ADMIN_PASSWORD@127.0.0.1:$RESTORE_PORT/$RESTORE_DB?sslmode=prefer"

echo "== 1. Резервная копия (pg_dump, формат custom) и проверка, что архив читается"
PGURL="$(url $ADMIN $PRIMARY)"
pg_dump "$PGURL" --format=custom --file=backups/course_app.dump
ls -lh backups/course_app.dump | awk '{print "файл:", $9, "размер:", $5}'
printf 'объектов в оглавлении архива (pg_restore --list): '; pg_restore --list backups/course_app.dump | grep -vc '^;'
# Роли хранятся на уровне кластера и в дамп БД не входят: сохраняем отдельно, БЕЗ паролей
pg_dumpall -d "$(url $ADMIN $PRIMARY require postgres)" --globals-only --no-role-passwords > backups/globals.sql
printf 'ролей в globals.sql: %s (пароли в файл не попадают)\n' "$(grep -c '^CREATE ROLE' backups/globals.sql)"

echo; echo "== 2. Чистый узел restore (профиль restore в compose шаблона)"
docker compose --profile restore rm -sfv restore >/dev/null 2>&1; docker volume rm -f sprint04_restore_data >/dev/null
docker compose --profile restore up -d restore >/dev/null 2>&1
until psql "$RURL" -X -Atq -c "SELECT 1" >/dev/null 2>&1; do sleep 1; done
printf 'таблиц в чистой БД до восстановления: '; psql "$RURL" -X -Atq -c "SELECT count(*) FROM pg_tables WHERE schemaname NOT IN ('pg_catalog','information_schema')"

echo; echo "== 3. Восстановление: сначала роли, потом данные (pg_restore)"
grep -v -E "ROLE $ADMIN\b" backups/globals.sql > backups/globals_clean.sql
psql "$(echo "$RURL" | sed "s#/$RESTORE_DB#/postgres#")" -X -q -f backups/globals_clean.sql 2>&1 | grep -v -E "already exists|^$" ; echo "роли созданы"
pg_restore --dbname="$RURL" backups/course_app.dump && echo "pg_restore завершён без ошибок"
printf 'RLS-политик в копии: '; psql "$RURL" -X -Atq -c "SELECT count(*) FROM pg_policies"
printf 'владелец таблицы trips: '; psql "$RURL" -X -Atq -c "SELECT tableowner FROM pg_tables WHERE tablename='trips'"

echo; echo "== 4. Проверка: число строк, контрольная сумма данных, ограничения"
printf '%-18s %10s %10s\n' таблица оригинал копия
for t in $TABLES; do
  a=$(qa $ADMIN $PRIMARY "SELECT count(*) FROM $t"); b=$(psql "$RURL" -X -Atq -c "SELECT count(*) FROM app.$t")
  printf '%-18s %10s %10s  %s\n' $t "$a" "$b" "$([ "$a" = "$b" ] && echo OK || echo РАЗЛИЧИЕ)"
done
A=$(psql "$PGURL" -X -At -f ../../lab3/sql/checksum.sql); B=$(psql "$RURL" -X -At -c "SET search_path=app" -f ../../lab3/sql/checksum.sql | tail -1)
echo "контрольная сумма оригинала: $A"; echo "контрольная сумма копии    : $B"
[ "$A" = "$B" ] && echo "СОВПАДАЕТ — данные восстановлены полностью" || echo "НЕ СОВПАДАЕТ"
printf 'внешних ключей (оригинал / копия): %s / %s\n' "$(qa $ADMIN $PRIMARY "SELECT count(*) FROM pg_constraint WHERE contype='f'")" "$(psql "$RURL" -X -Atq -c "SELECT count(*) FROM pg_constraint WHERE contype='f'")"
echo; echo "== 5. Уборка: узел restore останавливается и удаляется"
docker compose --profile restore rm -sfv restore >/dev/null 2>&1; docker volume rm -f sprint04_restore_data >/dev/null; echo "готово"
