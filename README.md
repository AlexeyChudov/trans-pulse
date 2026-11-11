# trans-pulse: система мониторинга общественного транспорта

REST API на Go (chi, чистый pgx, JWT) + PostgreSQL (основные данные) + MongoDB (телеметрия, журналы, карточки инцидентов, остановки с гео-поиском).

## Быстрый старт (нужен только Docker)

```bash
git clone https://github.com/AlexeyChudov/trans-pulse.git
cd trans-pulse
git checkout lab5          # ветка с полным стеком
make up                    # создаст .env со случайными паролями, соберёт и запустит всё
```

Без `make` (например, в Windows без WSL) те же шаги:

```bash
bash scripts/init_env.sh
docker compose -f docker-compose.app.yml up -d --build --wait
```

`make up` запускает три контейнера: PostgreSQL, MongoDB и API. Миграции применяются автоматически, первый администратор создаётся при первом запуске.

| Что | Где |
|---|---|
| API | http://localhost:8080 (проверка: `curl localhost:8080/health`) |
| Логин и пароль администратора | в файле `.env` (`ADMIN_LOGIN`, `ADMIN_PASSWORD`) |
| PostgreSQL на вашей машине | `localhost:55432`, пользователь `postgres`, пароль `POSTGRES_PASSWORD` из `.env` |
| MongoDB на вашей машине | `localhost:57017`, пользователь `root`, пароль `MONGO_ROOT_PASSWORD` из `.env` |

Файл `.env` создаётся автоматически и **в git не попадает**: пароли у каждого свои.

## Что можно запустить

| Команда | Что делает |
|---|---|
| `make smoke` | живая проверка работающего API (вход, роли, правила рейсов, MongoDB); нужны `curl` и `jq` |
| `make test-docker` | все тесты (юнит и интеграционные на настоящих PostgreSQL и MongoDB) внутри контейнера |
| `make experiments NAME=jsonbvsmongo` | сравнение JSONB и MongoDB на одинаковых данных (результат — в консоль) |
| `make experiments NAME=embedgrowth` | эксперимент по росту встроенного массива |
| `make logs` | журнал контейнера API |
| `make down` | остановить (данные сохраняются; полностью очистить: `docker compose -f docker-compose.app.yml down -v`) |

Запуск без Docker для API (нужны Go и запущенные базы): `make api-run`, тесты с хоста: `make api-test`. SQL-каталог лабораторной 5 проверяется командой `make sql-check` (нужны `psql` и `goose`).

## Если что-то не запускается

| Сообщение | Причина и что делать |
|---|---|
| `required variable … is missing` / `запустите scripts/init_env.sh` | нет файла `.env`; выполните `bash scripts/init_env.sh` (или `make init-env`) |
| `port is already allocated` / `address already in use` | порт занят другой программой; поменяйте `API_PORT`, `POSTGRES_HOST_PORT` или `MONGO_HOST_PORT` в `.env` и повторите |
| `Cannot connect to the Docker daemon` | не запущен Docker Desktop |
| `no matching manifest for linux/…` | у образа нет сборки под вашу архитектуру; сообщите нам, добавим `platform` |
| `api` постоянно перезапускается | посмотрите `make logs`: чаще всего неверный пароль в `.env` после смены (пересоздайте тома: `docker compose -f docker-compose.app.yml down -v`) |

## Документация

- архитектура и API: `docs/api/architecture.md`;
- лабораторная 5 (SQL-каталог, модель данных MongoDB, решения): `docs/lab5/`;
- SQL-запросы: `lab5/sql/queries/`.
