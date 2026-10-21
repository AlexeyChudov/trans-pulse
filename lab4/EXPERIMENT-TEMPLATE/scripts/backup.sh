#!/bin/bash
# РЕЗЕРВНАЯ КОПИЯ: данные (pg_dump) и список ролей без паролей (pg_dumpall). Файлы в backups/.
cd "$(dirname "$0")/.."; source scripts/lib.sh
mkdir -p backups
pg_dump "$(url $ADMIN $PRIMARY)" --format=custom --file=backups/course_app.dump
# Роли хранятся на уровне сервера и в дамп БД не входят; пароли в файл не попадают
pg_dumpall -d "$(url $ADMIN $PRIMARY require postgres)" --globals-only --no-role-passwords > backups/globals.sql
ls -lh backups/course_app.dump backups/globals.sql | awk '{print "файл:", $9, "размер:", $5}'
printf 'объектов в оглавлении архива (pg_restore --list): '; pg_restore --list backups/course_app.dump | grep -vc '^;'
