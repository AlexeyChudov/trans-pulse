#!/bin/sh
set -eu

: "${PGDATA:=/var/lib/postgresql/data}"
: "${PRIMARY_HOST:=primary}"
: "${PRIMARY_DB_PORT:=5432}"
: "${POSTGRES_DB:?POSTGRES_DB is required}"
: "${POSTGRES_USER:?POSTGRES_USER is required}"
: "${REPLICATION_USER:?REPLICATION_USER is required}"
: "${REPLICATION_PASSWORD:?REPLICATION_PASSWORD is required}"
: "${REPLICA_APPLICATION_NAME:=course_replica}"

password_file=/var/lib/postgresql/.pgpass
escaped_replication_password=$(printf '%s' "$REPLICATION_PASSWORD" | sed 's/\\/\\\\/g; s/:/\\:/g')

mkdir -p "$PGDATA"
chown -R postgres:postgres "$PGDATA"
chmod 700 "$PGDATA"

umask 077
printf '%s:%s:*:%s:%s\n' \
  "$PRIMARY_HOST" \
  "$PRIMARY_DB_PORT" \
  "$REPLICATION_USER" \
  "$escaped_replication_password" > "$password_file"
chown postgres:postgres "$password_file"

echo "Waiting for primary at ${PRIMARY_HOST}:${PRIMARY_DB_PORT}..."
until pg_isready \
  --host "$PRIMARY_HOST" \
  --port "$PRIMARY_DB_PORT" \
  --username "$POSTGRES_USER" \
  --dbname "$POSTGRES_DB" >/dev/null 2>&1; do
  sleep 2
done

if [ ! -s "$PGDATA/PG_VERSION" ]; then
  if [ -n "$(find "$PGDATA" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
    echo "Replica data directory is not empty and has no PG_VERSION." >&2
    echo "Inspect the replica volume before performing an explicit clean reset." >&2
    exit 1
  fi

  echo "Creating replica from a primary base backup..."
  gosu postgres pg_basebackup \
    --dbname="host=${PRIMARY_HOST} port=${PRIMARY_DB_PORT} user=${REPLICATION_USER} passfile=${password_file} application_name=${REPLICA_APPLICATION_NAME}" \
    --pgdata="$PGDATA" \
    --format=plain \
    --wal-method=stream \
    --write-recovery-conf \
    --progress
fi

exec docker-entrypoint.sh postgres -c hot_standby=on
