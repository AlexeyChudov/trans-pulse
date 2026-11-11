DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/trans-pulse_db?sslmode=disable
MIGRATIONS_DIR=./migrations	

.PHONY: test test-db migration-test migration-up migration-down


test-db:
	TEST_DATABASE_URL="$(DATABASE_URL)" go test -v ./tests/...

migrate-test1:
	TEST_DATABASE_URL="$(DATABASE_URL)" ./tests/migrations1.sh

migrate-test2:
	TEST_DATABASE_URL="$(DATABASE_URL)" ./tests/migrations2.sh

migrate-up:
	goose -dir ./migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir ./migrations postgres "$(DATABASE_URL)" down


migrate-status:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" status

# ---- REST API ----
API_DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/trans-pulse_api?sslmode=disable
API_TEST_DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/trans-pulse_api_test?sslmode=disable

.PHONY: api-migrate api-run api-test create-admin mongo-up sql-check init-env up down logs smoke test-docker experiments

api-migrate:
	goose -dir ./migrations postgres "$(API_DATABASE_URL)" up

# Переменные (DATABASE_URL, JWT_SECRET, ...) берутся из .env (шаблон: .env.example)
api-run:
	set -a; . ./.env; set +a; go run ./cmd/api

# MongoDB для документной части (пароль из .env)
mongo-up:
	set -a; . ./.env; set +a; docker compose -f docker-compose.mongo.yml up -d

# Интеграционные тесты идут против API_TEST_DATABASE_URL (PostgreSQL) и API_TEST_MONGO_URI (MongoDB); без них они пропускаются
api-test:
	set -a; . ./.env; set +a; API_TEST_DATABASE_URL="$(API_TEST_DATABASE_URL)" API_TEST_MONGO_URI="$$MONGO_URI" go test -count=1 ./internal/...

# Автопроверка SQL-каталога на тестовых данных (лаб. работа 5)
sql-check:
	bash lab5/sql/check.sh

# make create-admin LOGIN=boss ADMIN_PASSWORD='длинный-пароль'
create-admin:
	DATABASE_URL="$(API_DATABASE_URL)" ADMIN_PASSWORD="$(ADMIN_PASSWORD)" go run ./cmd/createadmin -login "$(LOGIN)"

# ---- Запуск «из коробки» (нужен только Docker): см. README.md ----
COMPOSE = docker compose -f docker-compose.app.yml

# создать .env со случайными паролями (если его нет)
init-env:
	bash scripts/init_env.sh

# поднять PostgreSQL + MongoDB + API (миграции и первый администратор применяются автоматически)
up: init-env
	$(COMPOSE) up -d --build --wait
	@echo "API: http://localhost:$$(grep ^API_PORT= .env | cut -d= -f2)/health  |  администратор: логин и пароль в .env (ADMIN_LOGIN, ADMIN_PASSWORD)"

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f api

# живая проверка запущенного API (нужны curl и jq)
smoke:
	set -a; . ./.env; set +a; bash scripts/api_smoke.sh

# тесты (юнит и интеграционные на настоящих PostgreSQL и MongoDB) внутри контейнера
test-docker:
	$(COMPOSE) --profile tools run --rm --build tests

# эксперименты лабораторной 5 внутри контейнера: make experiments NAME=jsonbvsmongo (или embedgrowth)
experiments:
	$(COMPOSE) --profile tools run --rm --build tests go run ./cmd/$(NAME)
