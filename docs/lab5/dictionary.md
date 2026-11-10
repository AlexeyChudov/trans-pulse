# Словарь коллекций и полей

Тип — BSON-тип в базе. «Об.» — обязательное поле (проверяется схемой валидации). Целые идентификаторы хранятся как `long` (допускается и `int`).

## telemetry_buckets

| Поле | Тип | Об. | Описание и ограничения |
|---|---|:-:|---|
| `_id` | ObjectId | да | служебный ключ |
| `vehicle_id` | long | да | ссылка на `vehicles.id` |
| `hour` | date | да | начало часа (UTC); вместе с `vehicle_id` уникален |
| `count` | int | да | число точек в корзине, 0…720 |
| `points` | array | да | точки, не более 720 |
| `points[].ts` | date | да | время измерения (BR-12) |
| `points[].lat` | number | да | широта, −90…90 (BR-10) |
| `points[].lon` | number | да | долгота, −180…180 (BR-11) |
| `points[].speed` | number | да | скорость, км/ч, 0…250 |
| `points[].trip_id` | long | нет | рейс, на котором находился автобус |
| `schema_version` | int | да | версия схемы документа, сейчас 1 |

## trip_journals

| Поле | Тип | Об. | Описание и ограничения |
|---|---|:-:|---|
| `_id` | long | да | номер рейса (`trips.id`) |
| `events` | array | да | события в хронологическом порядке, не более 500 |
| `events[].at` | date | да | время события |
| `events[].type` | string | да | `DEPARTED`, `ARRIVED_AT_STOP`, `DELAY_DETECTED`, `TELEMETRY_LOST`, `TELEMETRY_RESTORED`, `NOTE` |
| `events[].stop_id` | long | для `ARRIVED_AT_STOP` | ссылка на `stops.id` |
| `events[].delay_seconds` | int | для `DELAY_DETECTED` | задержка, 0…86400 с |
| `events[].note` | string | нет | заметка, до 500 символов |
| `last_event_at` | date / null | да | время последнего события (нужно для проверки хронологии) |
| `created_at`, `updated_at` | date | да | служебные времена |
| `schema_version` | int | да | сейчас 1 |

## incident_cases

| Поле | Тип | Об. | Описание и ограничения |
|---|---|:-:|---|
| `_id` | long | да | номер инцидента (`incidents.id`) |
| `trip_id`, `vehicle_id`, `route_id` | long | да | ссылки на PostgreSQL, заполняются при создании из данных инцидента |
| `title` | string | да | заголовок, 3–200 символов |
| `tags` | array of string | да | не более 10 уникальных тегов по 1–30 символов, нижний регистр |
| `status` | string | да | `OPEN` или `CLOSED` |
| `comments` | array | да | не более 200 |
| `comments[].author_id` | long | да | ссылка на `users.id` (берётся из токена) |
| `comments[].text` | string | да | 1–2000 символов |
| `comments[].at` | date | да | время комментария |
| `created_by` | long | да | автор карточки (`users.id`) |
| `created_at`, `updated_at` | date | да | служебные времена |
| `closed_at`, `closed_by` | date, long | при `CLOSED` | обязательны для закрытой карточки и запрещены для открытой |
| `schema_version` | int | да | сейчас 1 |

## stop_points

| Поле | Тип | Об. | Описание и ограничения |
|---|---|:-:|---|
| `_id` | long | да | номер остановки (`stops.id`) |
| `name` | string | да | копия имени из PostgreSQL, 1–150 символов |
| `location` | object (GeoJSON Point) | да | `{type: "Point", coordinates: [долгота, широта]}`; индекс `2dsphere` |
| `route_ids` | array of long | да | обратные ссылки на маршруты (M:N), уникальные |
| `attributes` | object | нет | гибкие атрибуты (не более 20); значения — строка, число или логическое |
| `updated_at` | date | да | служебное время |
| `schema_version` | int | да | сейчас 1 |

## route_profiles

| Поле | Тип | Об. | Описание и ограничения |
|---|---|:-:|---|
| `_id` | long | да | номер маршрута (`routes.id`) |
| `stops` | array | да | остановки маршрута, не более 200 |
| `stops[].stop_id` | long | да | ссылка на `stop_points._id` |
| `stops[].seq` | int | да | порядковый номер ≥ 1 (строго 1…N, проверяет сервис) |
| `updated_at` | date | да | служебное время |
| `schema_version` | int | да | сейчас 1 |
