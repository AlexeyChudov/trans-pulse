#!/bin/bash
# Замер запросов: ./lab3/bench.sh <label> q1 q2 ...
# 7 запусков EXPLAIN (ANALYZE, BUFFERS); в lab3/explain/<q>_<label>.txt сохраняется план МЕДИАННОГО запуска,
# сводка -> lab3/results/bench_<label>.txt
set -e
DB="${LAB3_DATABASE_URL:-postgres://postgres:postgres@localhost:5432/trans-pulse_lab3?sslmode=disable}"
LABEL=$1; shift
RUNS=${RUNS:-7}
DIR=$(cd "$(dirname "$0")" && pwd)
OUT="$DIR/results/bench_$LABEL.txt"
TMP=$(mktemp -d)
: > "$OUT"
for q in "$@"; do
  f=$(ls "$DIR"/sql/queries/${q}_*.sql)
  for i in $(seq $RUNS); do
    (echo '\set QUIET 1'; grep '^\\set' "$f"; echo "EXPLAIN (ANALYZE, BUFFERS)"; grep -v '^\\set' "$f" | grep -v '^--') | psql "$DB" > "$TMP/$i.txt"
    echo "$(grep 'Execution Time' "$TMP/$i.txt" | awk '{print $3}') $i" >> "$TMP/times"
  done
  sorted=$(sort -n "$TMP/times")
  med_i=$(echo "$sorted" | sed -n "$((RUNS/2+1))p" | awk '{print $2}')
  median=$(echo "$sorted" | sed -n "$((RUNS/2+1))p" | awk '{print $1}')
  cp "$TMP/$med_i.txt" "$DIR/explain/${q}_$LABEL.txt"
  bufs=$(grep -m1 'Buffers:' "$DIR/explain/${q}_$LABEL.txt" | sed 's/^ *//')
  echo "$q $LABEL median_ms=$median min_ms=$(echo "$sorted" | head -1 | awk '{print $1}') max_ms=$(echo "$sorted" | tail -1 | awk '{print $1}') runs=[$(awk '{printf "%s ", $1}' "$TMP/times" | sed 's/ $//')] $bufs" | tee -a "$OUT"
  rm -f "$TMP/times"
done
rm -rf "$TMP"
