#!/bin/bash

set -e

DB_URL="${TEST_DATABASE_URL:?TEST_DATABASE_URL is not set}"

echo "Reset test database"

psql "$DB_URL" <<EOF
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
EOF

echo "Run all migrations"

goose -dir ./migrations postgres "$DB_URL" up

echo "Check migration status"

goose -dir ./migrations postgres "$DB_URL" status

echo "Run schema and constraint tests"

TEST_DATABASE_URL="$DB_URL" go test ./tests/...

echo "Migration test passed"