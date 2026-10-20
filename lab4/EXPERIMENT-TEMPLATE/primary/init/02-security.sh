#!/bin/sh
# Дополнение к шаблону: SSL, обязательное шифрование, search_path ролей, журнал подключений.
# Запускается один раз при создании primary (реплика получает те же настройки через pg_basebackup).
set -eu

# 1. Самоподписанный сертификат (хранится только внутри тома данных, в git его нет)
openssl req -new -x509 -days 365 -nodes -subj "/CN=sprint04-postgres" \
  -out "$PGDATA/server.crt" -keyout "$PGDATA/server.key" 2>/dev/null
chmod 600 "$PGDATA/server.key"

cat >> "$PGDATA/postgresql.conf" <<CONF
ssl = on
log_connections = on
log_disconnections = on
log_statement = 'ddl'
cluster_name = 'primary'
CONF

# 2. pg_hba.conf: подключения по сети без SSL к обычным базам отклоняются.
#    Правила проверяются сверху вниз, поэтому новое правило ставим первым.
{ echo "# отказ любому подключению по сети без SSL (кроме служебной репликации)"
  echo "hostnossl all all all reject"
  cat "$PGDATA/pg_hba.conf"; } > "$PGDATA/pg_hba.conf.new"
mv "$PGDATA/pg_hba.conf.new" "$PGDATA/pg_hba.conf"

# 3. Роли работают в схеме app (там лежат таблицы проекта)
psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" --set=ON_ERROR_STOP=1 \
  --set=admin="$POSTGRES_USER" --set=app="$APPLICATION_USER" --set=analyst="$ANALYST_USER" <<'SQL'
ALTER ROLE :"admin"   SET search_path = app, public;
ALTER ROLE :"app"     SET search_path = app, public;
ALTER ROLE :"analyst" SET search_path = app, public;
SQL
