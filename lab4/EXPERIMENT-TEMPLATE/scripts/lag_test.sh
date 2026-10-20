#!/bin/bash
# Замер отставания реплики (replication lag) при массовой загрузке 3 млн рейсов на primary.
cd "$(dirname "$0")/.."; source scripts/lib.sh
CSV=results/lag_samples.csv
echo "t_sec,lag_bytes,replay_lag_sec" > $CSV
START=$(date +%s)
( while true; do
    qa $ADMIN $PRIMARY "SELECT $(( $(date +%s) - START )) || ',' || pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn)::bigint || ',' || coalesce(round(extract(epoch FROM replay_lag)::numeric, 3), 0) FROM pg_stat_replication" >> $CSV
    sleep 0.5
  done ) & SAMPLER=$!
trap 'kill $SAMPLER 2>/dev/null' EXIT
echo "[$(date +%T)] Массовая загрузка 3 млн строк на primary (cmd/gen --mode load)"
LAB3_DATABASE_URL="$(url $ADMIN $PRIMARY)" go run ../../cmd/gen --mode load --seed 42 2>&1 | tail -3
LOADEND=$(( $(date +%s) - START ))
echo "[$(date +%T)] Загрузка закончилась на $LOADEND-й секунде, ждём пока replica догонит"
until [ "$(qa $ADMIN $PRIMARY "SELECT pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn)::bigint FROM pg_stat_replication")" = "0" ]; do sleep 0.5; done
CATCH=$(( $(date +%s) - START ))
kill $SAMPLER 2>/dev/null; wait $SAMPLER 2>/dev/null
echo "[$(date +%T)] replica догнала на $CATCH-й секунде (через $((CATCH-LOADEND)) с после конца загрузки)"
echo; echo "== Итог по замерам ($(($(wc -l < $CSV)-1)) замеров, файл $CSV)"
awk -F, 'NR>1 && $2!="" {n++; s+=$2; if($2>mb)mb=$2; if($3>ml)ml=$3} END {printf "макс. отставание: %.1f МБ; среднее: %.1f МБ; макс. replay_lag: %.2f с\n", mb/1048576, s/n/1048576, ml}' $CSV
printf 'строк trips: primary=%s replica=%s\n' "$(qa $ADMIN $PRIMARY 'SELECT count(*) FROM trips')" "$(qa $ADMIN $REPLICA 'SELECT count(*) FROM trips')"
