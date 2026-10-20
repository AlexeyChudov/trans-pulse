#!/bin/bash

DATABASE_URL="postgres://postgres:postgres@localhost:5432/trans-pulse_db?sslmode=disable"

pg_dump \
    "$DATABASE_URL" \
    --format=custom\
    --file=backup.dump
