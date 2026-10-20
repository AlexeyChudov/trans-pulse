#!/bin/bash


pg_restore \
    --host=localhost \
    --port=5433 \
    --username=postgres \
    --dbname=trans-pulse_db_restore \
    --clean \
    --if-exists \
    --no-owner \
    backup.dump
