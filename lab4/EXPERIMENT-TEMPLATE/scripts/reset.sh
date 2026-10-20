#!/bin/bash
# Полный сброс стенда (контейнеры и тома удаляются) и новая установка.
cd "$(dirname "$0")/.."
docker compose --profile restore down -v 2>&1 | tail -2
./scripts/setup.sh
