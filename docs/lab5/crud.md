# CRUD-сценарии документных коллекций (REST API)

Всё через HTTP API Go-приложения (`/api/v1`), JWT в заголовке `Authorization: Bearer …`. Роли: **D** — диспетчер или администратор, **A** — только администратор, **—** — без токена. Проверка всех сценариев вживую — `scripts/api_smoke.sh` (раздел MongoDB).

## Матрица CRUD

| Коллекция | Create | Read | Update | Delete |
|---|---|---|---|---|
| `telemetry_buckets` | `POST /telemetry` (D) — точка попадает в корзину автобуса и часа | `GET /vehicles/{id}/telemetry?from=&to=&limit=` (D) | не предусмотрено: телеметрия только добавляется | не предусмотрено (хранение ограничивается сроком жизни данных) |
| `trip_journals` | `POST /trips/{id}/journal` (D) | `GET /trips/{id}/journal` (D) | `POST /trips/{id}/journal/events` (D) — добавить событие | `DELETE /trips/{id}/journal` (A) |
| `incident_cases` | `POST /incidents/{id}/case` (D) | `GET /incidents/{id}/case` (D) | `PATCH /incidents/{id}/case` (D) — заголовок, теги; `POST …/case/comments` (D); `POST …/case/close` (D) | `DELETE /incidents/{id}/case` (A) |
| `stop_points` | `POST /stop-points` (A) | `GET /stop-points/{id}` (—); `GET /stop-points/near?lat=&lon=&radius=&limit=` (—) | `PUT /stop-points/{id}` (A) — атрибуты | `DELETE /stop-points/{id}` (A) |
| `route_profiles` | `PUT /route-profiles/{id}/stops` (A) — задать состав маршрута | `GET /route-profiles/{id}` (—) | `PUT /route-profiles/{id}/stops` (A) — заменить состав | `DELETE /route-profiles/{id}` (A) |

Телеметрия не обновляется и не удаляется по замыслу (поток событий только на добавление); у журнала правится только «хвост» (добавление события), прошлые события неизменны.

## Примеры

```bash
# вход и токен
TOKEN=$(curl -s localhost:8080/api/v1/auth/login -d '{"login":"ivan","password":"…"}' | jq -r .token)

# телеметрия: приём точки и история за период
curl -X POST localhost:8080/api/v1/telemetry -H "Authorization: Bearer $TOKEN" \
  -d '{"vehicle_id":2,"trip_id":4,"ts":"2026-10-01T08:00:10Z","latitude":50.4501,"longitude":30.5234,"speed_kmh":32.5}'
curl "localhost:8080/api/v1/vehicles/2/telemetry?from=2026-10-01T08:00:00Z&to=2026-10-01T09:00:00Z" -H "Authorization: Bearer $TOKEN"

# журнал рейса: создать, добавить событие, прочитать, удалить (админ)
curl -X POST localhost:8080/api/v1/trips/4/journal -H "Authorization: Bearer $TOKEN"
curl -X POST localhost:8080/api/v1/trips/4/journal/events -H "Authorization: Bearer $TOKEN" \
  -d '{"at":"2026-10-01T08:06:00Z","type":"DELAY_DETECTED","delay_seconds":360}'
curl localhost:8080/api/v1/trips/4/journal -H "Authorization: Bearer $TOKEN"

# карточка инцидента: создать, прокомментировать, поправить теги, закрыть
curl -X POST localhost:8080/api/v1/incidents/2/case -H "Authorization: Bearer $TOKEN" -d '{"title":"Разбор задержки","tags":["delay"]}'
curl -X POST localhost:8080/api/v1/incidents/2/case/comments -H "Authorization: Bearer $TOKEN" -d '{"text":"Водитель сообщил о пробке"}'
curl -X PATCH localhost:8080/api/v1/incidents/2/case -H "Authorization: Bearer $TOKEN" -d '{"tags":["delay","traffic"]}'
curl -X POST localhost:8080/api/v1/incidents/2/case/close -H "Authorization: Bearer $TOKEN"

# остановки: проекция, гео-поиск, состав маршрута (M:N)
curl -X POST localhost:8080/api/v1/stop-points -H "Authorization: Bearer $ADMIN" -d '{"stop_id":1,"attributes":{"shelter":true,"bench_count":2}}'
curl "localhost:8080/api/v1/stop-points/near?lat=50.45&lon=30.52&radius=2000&limit=5"
curl -X PUT localhost:8080/api/v1/route-profiles/3/stops -H "Authorization: Bearer $ADMIN" -d '{"stops":[{"stop_id":1,"seq":1},{"stop_id":2,"seq":2}]}'
```

## Ответы при ошибках (единый формат `{"error":{"code","message"}}`)

| Ситуация | HTTP | code |
|---|---|---|
| данные не прошли проверку (диапазоны, типы, обязательные поля) | 400 | `invalid_input` |
| нет токена или токен неверный | 401 | `unauthorized` |
| роли не хватает (например, диспетчер удаляет журнал) | 403 | `forbidden` |
| родителя или документа нет (журнал, карточка, остановка, рейс) | 404 | `not_found` |
| нарушены правила: дубликат, событие раньше последнего, журнал или корзина заполнены, закрытая карточка, остановка входит в маршруты | 409 | `conflict` |
