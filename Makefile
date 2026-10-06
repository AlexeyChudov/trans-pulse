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


# ---- ЛР3 ----
LAB3_DB_URL ?= postgres://postgres:postgres@localhost:5432/trans-pulse_lab3?sslmode=disable

.PHONY: lab3-db gen-dev gen-load lab3-stats lab3-race lab3-deadlock lab3-loadbench

lab3-db:
	psql "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable" -c 'CREATE DATABASE "trans-pulse_lab3"'
	goose -dir ./migrations postgres "$(LAB3_DB_URL)" up

gen-dev:
	LAB3_DATABASE_URL="$(LAB3_DB_URL)" go run ./cmd/gen --mode dev --seed 42

gen-load:
	LAB3_DATABASE_URL="$(LAB3_DB_URL)" go run ./cmd/gen --mode load --seed 42

lab3-stats:
	psql "$(LAB3_DB_URL)" -f lab3/sql/stats.sql

lab3-race:
	LAB3_DATABASE_URL="$(LAB3_DB_URL)" go run ./cmd/lab3 race

lab3-deadlock:
	LAB3_DATABASE_URL="$(LAB3_DB_URL)" go run ./cmd/lab3 deadlock

lab3-loadbench:
	LAB3_DATABASE_URL="$(LAB3_DB_URL)" go run ./cmd/lab3 loadbench
