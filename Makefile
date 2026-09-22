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

