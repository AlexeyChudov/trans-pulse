#!/bin/sh
# Выполняется один раз при создании тома PostgreSQL: отдельная БД для интеграционных тестов.
set -eu
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c 'CREATE DATABASE "trans-pulse_api_test"'
