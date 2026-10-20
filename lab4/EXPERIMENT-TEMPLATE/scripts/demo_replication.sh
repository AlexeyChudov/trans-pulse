#!/bin/bash
# Репликация и её направление. Используются SQL-пробы шаблона (sql/01..05).
cd "$(dirname "$0")/.."; source scripts/lib.sh
run() { psql "$(url "$1" "$2")" -X -f "$3" 2>&1; }   # run <роль> <порт> <файл>
echo "== 1. Кто primary, кто replica (is_in_recovery: f = primary, t = replica)"
printf 'primary (порт %s): ' $PRIMARY; qa $ADMIN $PRIMARY "SELECT pg_is_in_recovery()"
printf 'replica (порт %s): ' $REPLICA; qa $ADMIN $REPLICA "SELECT pg_is_in_recovery()"
echo; echo "== 2. Статус на primary (шаблон: sql/01-primary-status.sql)"; run $ADMIN $PRIMARY sql/01-primary-status.sql
echo "== 3. Запись пробы на primary от имени приложения (sql/02-create-probe.sql)"; run $APP $PRIMARY sql/02-create-probe.sql
sleep 1
echo "== 4. Статус на replica (sql/03-replica-status.sql)"; run $ADMIN $REPLICA sql/03-replica-status.sql
echo "== 5. Чтение проб на replica: новая запись пришла (sql/04-read-probes.sql)"; run $ANALYST $REPLICA sql/04-read-probes.sql | head -8
echo "== 6. Запись напрямую на replica запрещена"
q $APP $REPLICA "INSERT INTO app.replication_probe (marker) VALUES ('direct-write')" | head -2
echo; echo "== 7. Сверка числа строк на обоих узлах"
for t in routes stops vehicles trips incidents replication_probe; do
  printf '%-18s primary=%-8s replica=%s\n' $t "$(qa $ADMIN $PRIMARY "SELECT count(*) FROM $t")" "$(qa $ADMIN $REPLICA "SELECT count(*) FROM $t")"
done
