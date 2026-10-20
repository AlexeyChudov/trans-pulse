#!/bin/bash
# RLS: диспетчер закрывает инцидент только от своего имени. Аудит: кто и что изменил.
cd "$(dirname "$0")/.."; source scripts/lib.sh
P=$PRIMARY
INC=$(qa $ADMIN $P "SELECT id FROM incidents WHERE status='OPEN' ORDER BY id LIMIT 1")
RES=$(qa $ADMIN $P "SELECT id FROM incidents WHERE status='RESOLVED' ORDER BY id LIMIT 1")
echo "Открытый инцидент для опыта: $INC; уже закрытый: $RES. Приложение работает от имени диспетчера с id=3 (user03)."
echo; echo "== RLS-1. Закрыть инцидент от ЧУЖОГО имени (resolved_by = 4)"
try $APP $P "SET app.user_id = '3'; UPDATE incidents SET status='RESOLVED', resolved_at=now(), resolved_by_user_id=4 WHERE id=$INC"
echo "== RLS-2. Закрыть инцидент, не сообщив, кто пользователь (app.user_id не задан)"
try $APP $P "UPDATE incidents SET status='RESOLVED', resolved_at=now(), resolved_by_user_id=3 WHERE id=$INC"
echo "== RLS-3. Создать инцидент сразу в статусе RESOLVED"
try $APP $P "INSERT INTO incidents (trip_id,type,status,resolved_at,resolved_by_user_id) VALUES (1,'DELAY','RESOLVED',now(),3)"
echo "== RLS-4. Изменить уже закрытый инцидент"
printf '  затронуто строк: '; psql "$(url $APP $P)" -X -Atq -c "SET app.user_id='3'; WITH u AS (UPDATE incidents SET description='hack' WHERE id=$RES RETURNING 1) SELECT count(*) FROM u" 2>&1 | tail -1
echo "== RLS-5. Закрыть инцидент от СВОЕГО имени (resolved_by = 3) — разрешено"
try $APP $P "SET app.user_id = '3'; UPDATE incidents SET status='RESOLVED', resolved_at=now(), resolved_by_user_id=3 WHERE id=$INC"
echo "== RLS-6. Аналитик инцидент прочитать может, изменить нет"
try $ANALYST $P "SELECT count(*) FROM incidents WHERE id=$INC"
try $ANALYST $P "UPDATE incidents SET description='x' WHERE id=$INC"

echo; echo "== Аудит. Приложение (диспетчер 3) делает критические операции"
VEH=$(qa $ADMIN $P "SELECT id FROM vehicles WHERE status='IN_SERVICE' ORDER BY id DESC LIMIT 1")
TRIP=$(qa $ADMIN $P "SELECT id FROM trips WHERE status='PLANNED' ORDER BY id LIMIT 1")
try $APP $P "SET app.user_id='3'; UPDATE vehicles SET status='OUT_OF_SERVICE' WHERE id=$VEH"
try $APP $P "SET app.user_id='3'; UPDATE trips SET status='CANCELLED' WHERE id=$TRIP"
echo; echo "== Журнал audit_log (читает администратор)"
q $ADMIN $P "SELECT id, to_char(at,'HH24:MI:SS') AS time, db_user, app_user, table_name, row_id, action, old_value, new_value FROM audit_log ORDER BY id"
echo "== Приложение изменить или стереть журнал не может"
try $APP $P "DELETE FROM audit_log"
try $APP $P "UPDATE audit_log SET app_user='9'"
