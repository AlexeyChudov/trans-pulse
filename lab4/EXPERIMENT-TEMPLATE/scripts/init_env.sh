#!/bin/bash
# Создаёт .env из .env.example и подставляет случайные пароли (если .env ещё нет).
set -e
cd "$(dirname "$0")/.."
if [ -f .env ]; then echo ".env уже существует, не трогаю"; exit 0; fi
while IFS= read -r line; do
  case "$line" in
    *PASSWORD=*) echo "${line%%=*}=$(openssl rand -hex 16)" ;;
    *) echo "$line" ;;
  esac
done < .env.example > .env
chmod 600 .env
echo ".env создан (права 600), пароли случайные"
