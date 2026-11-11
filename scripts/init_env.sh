#!/bin/bash
# Создаёт .env из .env.example со случайными паролями (если .env ещё нет). Безопасно запускать повторно.
set -e
cd "$(dirname "$0")/.."
if [ -f .env ]; then echo ".env уже существует, не трогаю"; exit 0; fi
rand() { openssl rand -hex 16; }
PG=$(rand); MONGO=$(rand); JWT=$(openssl rand -hex 32); ADMIN=$(rand)
sed -e "s/CHANGE_ME_PG/$PG/g" -e "s/CHANGE_ME_MONGO/$MONGO/g" -e "s/CHANGE_ME_JWT/$JWT/g" -e "s/CHANGE_ME_ADMIN/$ADMIN/g" .env.example > .env
chmod 600 .env
echo ".env создан со случайными паролями (логин администратора API: $(grep ^ADMIN_LOGIN= .env | cut -d= -f2), пароль — в .env)"
