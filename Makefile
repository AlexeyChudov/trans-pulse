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

.PHONY: api-migrate api-run api-test create-admin

api-migrate:
	goose -dir ./migrations postgres "$(API_DATABASE_URL)" up

# Переменные (DATABASE_URL, JWT_SECRET, ...) берутся из .env (шаблон: .env.example)
api-run:
	set -a; . ./.env; set +a; go run ./cmd/api

# Интеграционные тесты репозиториев идут против API_TEST_DATABASE_URL (без неё они пропускаются)
api-test:
	API_TEST_DATABASE_URL="$(API_TEST_DATABASE_URL)" go test -count=1 ./internal/...

# make create-admin LOGIN=boss ADMIN_PASSWORD='длинный-пароль'
create-admin:
	DATABASE_URL="$(API_DATABASE_URL)" ADMIN_PASSWORD="$(ADMIN_PASSWORD)" go run ./cmd/createadmin -login "$(LOGIN)"
