#!/bin/bash
# Поднять стенд из шаблона (primary + replica), применить миграции проекта, залить данные, настроить права/RLS/аудит.
set -e
cd "$(dirname "$0")/.."
[ -f .env ] || ./scripts/init_env.sh
source scripts/lib.sh
docker compose up -d primary replica
wait_ready $PRIMARY && wait_ready $REPLICA
echo "== миграции проекта (goose) от имени $ADMIN; роли создаёт шаблон, поэтому миграцию ролей не применяем"
goose -dir ../../migrations postgres "$(url "$ADMIN" $PRIMARY)" up-to 20260921132411
echo "== данные (режим ${GEN_MODE:-dev})"
LAB3_DATABASE_URL="$(url "$ADMIN" $PRIMARY)" go run ../../cmd/gen --mode "${GEN_MODE:-dev}" --seed 42
echo "== права, RLS, аудит"
psql "$(url "$ADMIN" $PRIMARY)" -X -q -v ON_ERROR_STOP=1 -f sql/10-post-migration.sql 2>&1 | grep -v NOTICE || true
echo "== готово"
