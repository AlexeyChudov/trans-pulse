# Общие функции демо-скриптов. Подключение: source scripts/lib.sh (из bash)
LAB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
set -a; . "$LAB_DIR/.env"; set +a
PRIMARY=${PRIMARY_PORT:-55432}; REPLICA=${REPLICA_PORT:-55433}
PROJECT=sprint-04-postgresql

pw() { case "$1" in
  "$POSTGRES_ADMIN_USER") echo "$POSTGRES_ADMIN_PASSWORD";; "$APPLICATION_USER") echo "$APPLICATION_PASSWORD";;
  "$ANALYST_USER") echo "$ANALYST_PASSWORD";; "$REPLICATION_USER") echo "$REPLICATION_PASSWORD";; esac; }
ADMIN=$POSTGRES_ADMIN_USER; APP=$APPLICATION_USER; ANALYST=$ANALYST_USER

# url <роль> <порт> [sslmode] [база]
url() { echo "postgresql://$1:$(pw "$1")@127.0.0.1:$2/${4:-$POSTGRES_DB}?sslmode=${3:-require}"; }
q()  { psql "$(url "$1" "$2")" -X -c "$3" 2>&1; }          # с форматированной таблицей
qa() { psql "$(url "$1" "$2")" -X -Atq -c "$3" 2>&1; }      # только значение
try() { # try <роль> <порт> <sql>: показать, разрешена ли операция
  local out
  if out=$(psql "$(url "$1" "$2")" -X -Atq -c "$3" 2>&1); then
    printf '  %-14s %-58s -> РАЗРЕШЕНО\n' "$1" "$3"
  else
    printf '  %-14s %-58s -> ЗАПРЕЩЕНО (%s)\n' "$1" "$3" "$(echo "$out" | head -1 | sed 's/^ERROR:  //')"
  fi
}
wait_ready() { # ждём, пока удастся реально войти (не только открыт порт)
  for i in $(seq 1 180); do
    psql "$(url "$ADMIN" "$1")" -X -Atq -c "SELECT 1" >/dev/null 2>&1 && return 0; sleep 1
  done; return 1; }
