#!/bin/sh
# Точка входа контейнера api: миграции БД, первый администратор (если задан), запуск сервера.
set -eu

echo "Применяем миграции PostgreSQL…"
goose -dir /migrations postgres "$DATABASE_URL" up

if [ -n "${ADMIN_LOGIN:-}" ] && [ -n "${ADMIN_PASSWORD:-}" ]; then
  # Если администратор уже есть (повторный запуск), это не ошибка.
  if out=$(createadmin -login "$ADMIN_LOGIN" 2>&1); then
    echo "$out"
  else
    case "$out" in
      *"конфликт"*) echo "Администратор $ADMIN_LOGIN уже существует." ;;
      *) echo "$out" >&2; exit 1 ;;
    esac
  fi
fi

exec api
