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

echo; [ $FAILED = 0 ] && echo "ВСЕ ПРОВЕРКИ ПРОЙДЕНЫ" || { echo "ЕСТЬ ОШИБКИ"; exit 1; }
