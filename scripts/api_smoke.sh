#!/bin/bash
# Живая проверка API через curl: вход, роли, бизнес-правила. Сервер должен быть запущен (make api-run),
# администратор создан (make create-admin LOGIN=boss ADMIN_PASSWORD=...).
#   ADMIN_LOGIN=boss ADMIN_PASSWORD=... bash scripts/api_smoke.sh
set -u
BASE=${BASE:-http://localhost:8080/api/v1}
ADMIN_LOGIN=${ADMIN_LOGIN:-boss}
: "${ADMIN_PASSWORD:?задайте ADMIN_PASSWORD}"
DISP_LOGIN="disp$RANDOM"; DISP_PASSWORD="disp-password-$RANDOM"
FAILED=0

# call <метод> <путь> <токен|-> [тело]  → печатает «HTTP-код» и тело; тело сохраняется в $BODY, код в $CODE
call() {
  local method=$1 path=$2 token=$3 data=${4:-}
  local args=(-s -o /tmp/api_smoke_body -w '%{http_code}' -X "$method" "$BASE$path" -H 'Content-Type: application/json')
  [ "$token" != "-" ] && args+=(-H "Authorization: Bearer $token")
  [ -n "$data" ] && args+=(-d "$data")
  CODE=$(curl "${args[@]}"); BODY=$(cat /tmp/api_smoke_body)
}
# expect <описание> <ожидаемый код>
expect() {
  if [ "$CODE" = "$2" ]; then printf '  ✓ %-62s %s\n' "$1" "$CODE"; else printf '  ✗ %-62s ожидался %s, получен %s: %s\n' "$1" "$2" "$CODE" "$BODY"; FAILED=1; fi
}

echo "== Вход и роли"
call POST /auth/register - "{\"login\":\"$DISP_LOGIN\",\"password\":\"$DISP_PASSWORD\"}"; expect "регистрация диспетчера" 201
DISP_ID=$(echo "$BODY" | jq .id)
call POST /auth/login - "{\"login\":\"$DISP_LOGIN\",\"password\":\"неверный-пароль\"}"; expect "вход с неверным паролем" 401
call POST /auth/login - "{\"login\":\"$DISP_LOGIN\",\"password\":\"$DISP_PASSWORD\"}"; expect "вход диспетчера" 200
DISP=$(echo "$BODY" | jq -r .token)
call POST /auth/login - "{\"login\":\"$ADMIN_LOGIN\",\"password\":\"$ADMIN_PASSWORD\"}"; expect "вход администратора" 200
ADMIN=$(echo "$BODY" | jq -r .token)
call GET /auth/me "$DISP"; expect "GET /auth/me (диспетчер)" 200
echo "$BODY" | jq -e 'has("password_hash") | not' >/dev/null && echo "  ✓ в ответе нет хэша пароля" || { echo "  ✗ в ответе есть хэш пароля"; FAILED=1; }
call GET /auth/me -; expect "GET /auth/me без токена" 401
call GET /auth/me "мусор"; expect "GET /auth/me с неверным токеном" 401
call POST /routes "$DISP" '{"number":"1","name":"x"}'; expect "диспетчер создаёт маршрут (нельзя)" 403

echo "== Маршруты и автобусы (администратор)"
NUM=$((RANDOM % 9000 + 100))
call POST /routes "$ADMIN" "{\"number\":\"$NUM\",\"name\":\"Центр — Вокзал\"}"; expect "создание маршрута" 201
ROUTE=$(echo "$BODY" | jq .id)
call POST /routes "$ADMIN" "{\"number\":\"$NUM\",\"name\":\"Дубль\"}"; expect "дубликат номера маршрута" 409
call POST /routes "$ADMIN" '{"number":"","name":"x"}'; expect "пустой номер маршрута" 400
call POST /routes "$ADMIN" '{"number":"5","name":"x","role":"ADMIN"}'; expect "неизвестное поле в теле" 400
call GET /routes -; expect "список маршрутов без токена (открытый)" 200
call GET "/routes/$ROUTE/stops" -; expect "остановки маршрута" 200
call GET /routes/999999 -; expect "несуществующий маршрут" 404
call POST /vehicles "$ADMIN" "{\"fleet_number\":\"S$RANDOM\",\"model\":\"MAN\"}"; expect "создание автобуса" 201
BUS=$(echo "$BODY" | jq .id)
call PATCH "/vehicles/$BUS" "$ADMIN" '{"status":"IN_SERVICE"}'; expect "REGISTERED → IN_SERVICE" 200
call PATCH "/vehicles/$BUS" "$ADMIN" '{"status":"REGISTERED"}'; expect "возврат в REGISTERED (нельзя)" 409

echo "== Рейсы: жизненный цикл и правило «один активный рейс на автобус»"
TRIP_BODY="{\"route_id\":$ROUTE,\"vehicle_id\":$BUS,\"planned_start\":\"2026-10-01T08:00:00Z\",\"planned_end\":\"2026-10-01T09:00:00Z\"}"
call POST /trips "$ADMIN" "$TRIP_BODY"; expect "создание рейса 1" 201
T1=$(echo "$BODY" | jq .id)
call POST /trips "$ADMIN" "$TRIP_BODY"; expect "создание рейса 2" 201
T2=$(echo "$BODY" | jq .id)
call POST /trips "$ADMIN" "{\"route_id\":$ROUTE,\"vehicle_id\":$BUS,\"planned_start\":\"2026-10-01T09:00:00Z\",\"planned_end\":\"2026-10-01T08:00:00Z\"}"; expect "конец раньше начала" 400
call POST "/trips/$T1/start" "$ADMIN"; expect "старт рейса 1" 200
echo "     статус: $(echo "$BODY" | jq -r .status), actual_start: $(echo "$BODY" | jq -r .actual_start)"
call POST "/trips/$T2/start" "$ADMIN"; expect "старт рейса 2 того же автобуса (BR-07)" 409
call POST "/trips/$T1/cancel" "$ADMIN"; expect "отмена идущего рейса (нельзя)" 409
call POST "/trips/$T1/complete" "$ADMIN"; expect "завершение рейса 1" 200
call POST "/trips/$T1/start" "$ADMIN"; expect "повторный старт завершённого рейса (BR-18)" 409
call POST "/trips/$T2/start" "$ADMIN"; expect "старт рейса 2 после завершения первого" 200
call GET "/trips?status=IN_PROGRESS&route_id=$ROUTE" "$DISP"; expect "диспетчер смотрит активные рейсы" 200
echo "     найдено рейсов: $(echo "$BODY" | jq length)"
call GET "/trips?limit=abc" "$DISP"; expect "limit=abc" 400
call DELETE "/routes/$ROUTE" "$ADMIN"; expect "удаление маршрута с рейсами (нельзя)" 409

echo "== Инциденты (создаются системой мониторинга; здесь добавляем строку напрямую в БД)"
if [ -n "${DATABASE_URL:-}" ]; then
  psql "$DATABASE_URL" -Atqc "INSERT INTO incidents (trip_id, type, description) VALUES ($T2, 'DELAY', 'smoke-test')" >/dev/null
  call GET "/incidents?status=OPEN&trip_id=$T2" "$DISP"; expect "список открытых инцидентов" 200
  INC=$(echo "$BODY" | jq '.[0].id')
  call POST "/incidents/$INC/resolve" "$DISP"; expect "диспетчер закрывает инцидент" 200
  echo "     закрыл пользователь #$(echo "$BODY" | jq .resolved_by_user_id) (диспетчер #$DISP_ID)"
  call POST "/incidents/$INC/resolve" "$DISP"; expect "повторное закрытие инцидента" 409
else
  echo "  (пропущено: задайте DATABASE_URL, чтобы создать тестовый инцидент)"
fi

echo "== MongoDB: телеметрия, журнал рейса, карточка инцидента, остановки и маршруты (M:N)"
if [ -n "${DATABASE_URL:-}" ]; then
  # --- телеметрия (автобус BUS существует в PostgreSQL)
  P="{\"vehicle_id\":$BUS,\"trip_id\":$T2,\"ts\":\"2026-10-01T08:00:10Z\",\"latitude\":50.4501,\"longitude\":30.5234,\"speed_kmh\":32.5}"
  call POST /telemetry "$DISP" "$P"; expect "приём GPS-точки" 201
  call POST /telemetry "$DISP" "{\"vehicle_id\":$BUS,\"ts\":\"2026-10-01T08:00:20Z\",\"latitude\":50.4510,\"longitude\":30.5240,\"speed_kmh\":35}"; expect "вторая точка того же часа" 201
  call POST /telemetry "$DISP" "{\"vehicle_id\":$BUS,\"ts\":\"2026-10-01T08:00:30Z\",\"latitude\":95,\"longitude\":30,\"speed_kmh\":10}"; expect "широта 95 (BR-10)" 400
  call POST /telemetry "$DISP" "{\"vehicle_id\":$BUS,\"latitude\":50,\"longitude\":30,\"speed_kmh\":10}"; expect "нет времени измерения (BR-12)" 400
  call POST /telemetry "$DISP" "{\"vehicle_id\":999999,\"ts\":\"2026-10-01T08:00:30Z\",\"latitude\":50,\"longitude\":30,\"speed_kmh\":10}"; expect "неизвестный автобус (BR-09)" 400
  call GET "/vehicles/$BUS/telemetry?from=2026-10-01T08:00:00Z&to=2026-10-01T09:00:00Z" "$DISP"; expect "история телеметрии" 200
  echo "     точек в истории: $(echo "$BODY" | jq length) (хранятся в одном документе-корзине)"
  call GET "/vehicles/$BUS/telemetry?from=2026-10-01T00:00:00Z&to=2026-10-03T00:00:00Z" "$DISP"; expect "период больше 24 часов" 400

  # --- журнал рейса (рейс T2 — IN_PROGRESS)
  call POST "/trips/$T2/journal" "$DISP"; expect "создание журнала рейса" 201
  call POST "/trips/$T2/journal" "$DISP"; expect "повторное создание журнала" 409
  call POST "/trips/$T2/journal/events" "$DISP" '{"at":"2026-10-01T08:00:00Z","type":"DEPARTED"}'; expect "событие DEPARTED" 201
  call POST "/trips/$T2/journal/events" "$DISP" '{"at":"2026-10-01T08:06:00Z","type":"DELAY_DETECTED","delay_seconds":360}'; expect "событие DELAY_DETECTED" 201
  call POST "/trips/$T2/journal/events" "$DISP" '{"at":"2026-10-01T08:03:00Z","type":"NOTE","note":"раньше последнего"}'; expect "событие раньше последнего (хронология)" 409
  call POST "/trips/$T2/journal/events" "$DISP" '{"at":"2026-10-01T08:07:00Z","type":"DELAY_DETECTED"}'; expect "DELAY_DETECTED без задержки" 400
  call GET "/trips/$T2/journal" "$DISP"; expect "чтение журнала" 200
  echo "     событий в журнале: $(echo "$BODY" | jq '.events | length')"
  call DELETE "/trips/$T2/journal" "$DISP"; expect "диспетчер удаляет журнал (нельзя)" 403
  call DELETE "/trips/$T2/journal" "$ADMIN"; expect "администратор удаляет журнал" 204

  # --- карточка инцидента: ссылки на PostgreSQL, комментарии встроены
  call POST "/incidents/$INC/case" "$DISP" '{"title":"Разбор задержки рейса","tags":["Delay","traffic"]}'; expect "создание карточки инцидента" 201
  echo "     ссылки из PostgreSQL: trip=$(echo "$BODY" | jq .trip_id) vehicle=$(echo "$BODY" | jq .vehicle_id) route=$(echo "$BODY" | jq .route_id); теги: $(echo "$BODY" | jq -c .tags)"
  call POST "/incidents/$INC/case/comments" "$DISP" '{"text":"Водитель сообщил о пробке"}'; expect "комментарий к карточке" 201
  call PATCH "/incidents/$INC/case" "$DISP" '{"tags":["delay","traffic","resolved"]}'; expect "правка тегов" 200
  call POST "/incidents/$INC/case/close" "$DISP"; expect "закрытие карточки (инцидент закрыт в PostgreSQL)" 200
  call POST "/incidents/$INC/case/comments" "$DISP" '{"text":"поздно"}'; expect "комментарий к закрытой карточке" 409
  OPEN_INC=$(psql "$DATABASE_URL" -Atqc "INSERT INTO incidents (trip_id, type, description) VALUES ($T1, 'NO_TELEMETRY', 'smoke-test-open') RETURNING id")
  call POST "/incidents/$OPEN_INC/case" "$DISP" '{"title":"Карточка открытого инцидента"}'; expect "карточка для открытого инцидента" 201
  call POST "/incidents/$OPEN_INC/case/close" "$DISP"; expect "закрыть карточку, пока инцидент OPEN в PostgreSQL" 409
  call POST "/incidents/999999/case" "$DISP" '{"title":"Нет такого инцидента"}'; expect "карточка несуществующего инцидента" 404

  # --- остановки (проекция PostgreSQL + геопоиск) и связь M:N «маршруты ↔ остановки»
  S1=$(psql "$DATABASE_URL" -Atqc "INSERT INTO stops (name, latitude, longitude) VALUES ('Центральная', 50.4501, 30.5234) RETURNING id")
  S2=$(psql "$DATABASE_URL" -Atqc "INSERT INTO stops (name, latitude, longitude) VALUES ('Вокзал', 50.4419, 30.5118) RETURNING id")
  S3=$(psql "$DATABASE_URL" -Atqc "INSERT INTO stops (name, latitude, longitude) VALUES ('Университет', 50.4489, 30.5145) RETURNING id")
  call POST /stop-points "$ADMIN" "{\"stop_id\":$S1,\"attributes\":{\"shelter\":true,\"bench_count\":2}}"; expect "проекция остановки 1 (с гибкими атрибутами)" 201
  call POST /stop-points "$ADMIN" "{\"stop_id\":$S2}"; expect "проекция остановки 2" 201
  call POST /stop-points "$ADMIN" "{\"stop_id\":$S3}"; expect "проекция остановки 3" 201
  call POST /stop-points "$ADMIN" "{\"stop_id\":$S3}"; expect "дубликат проекции" 409
  call POST /stop-points "$ADMIN" '{"stop_id":999999}'; expect "остановки нет в PostgreSQL" 400
  call POST /stop-points "$DISP" "{\"stop_id\":$S1}"; expect "диспетчер создаёт проекцию (нельзя)" 403
  call GET "/stop-points/near?lat=50.4500&lon=30.5230&radius=2000&limit=5" -; expect "геопоиск ближайших остановок (открытый)" 200
  echo "     найдено: $(echo "$BODY" | jq -c '[.[] | {name, distance_m: (.distance_m | floor)}]')"
  call GET "/stop-points/near?lat=50.45" -; expect "геопоиск без lon" 400
  call POST /routes "$ADMIN" "{\"number\":\"M$RANDOM\",\"name\":\"Второй маршрут\"}"; expect "второй маршрут" 201
  ROUTE2=$(echo "$BODY" | jq .id)
  call PUT "/route-profiles/$ROUTE/stops" "$ADMIN" "{\"stops\":[{\"stop_id\":$S1,\"seq\":1},{\"stop_id\":$S2,\"seq\":2}]}"; expect "состав маршрута 1: остановки 1, 2" 200
  call PUT "/route-profiles/$ROUTE2/stops" "$ADMIN" "{\"stops\":[{\"stop_id\":$S3,\"seq\":1},{\"stop_id\":$S1,\"seq\":2}]}"; expect "состав маршрута 2: остановки 3, 1 (M:N)" 200
  call GET "/stop-points/$S1" -; expect "остановка 1 входит в два маршрута" 200
  echo "     route_ids остановки 1: $(echo "$BODY" | jq -c '.route_ids | sort')"
  call PUT "/route-profiles/$ROUTE/stops" "$ADMIN" "{\"stops\":[{\"stop_id\":$S1,\"seq\":1},{\"stop_id\":$S2,\"seq\":3}]}"; expect "пропуск в нумерации остановок" 400
  call PUT "/route-profiles/$ROUTE/stops" "$ADMIN" "{\"stops\":[{\"stop_id\":$S1,\"seq\":1}]}"; expect "убрали остановку 2 из маршрута 1" 200
  call GET "/stop-points/$S2" -; echo "     route_ids остановки 2 после удаления из маршрута: $(echo "$BODY" | jq -c .route_ids)"
  call DELETE "/stop-points/$S1" "$ADMIN"; expect "удалить проекцию остановки, входящей в маршруты" 409
  call DELETE "/route-profiles/$ROUTE" "$ADMIN"; expect "удаление профиля маршрута 1" 204
  call DELETE "/route-profiles/$ROUTE2" "$ADMIN"; expect "удаление профиля маршрута 2" 204
  call DELETE "/stop-points/$S1" "$ADMIN"; expect "теперь проекцию можно удалить" 204
else
  echo "  (пропущено: задайте DATABASE_URL)"
fi

echo; [ $FAILED = 0 ] && echo "ВСЕ ПРОВЕРКИ ПРОЙДЕНЫ" || { echo "ЕСТЬ ОШИБКИ"; exit 1; }
