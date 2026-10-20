#!/bin/bash
# Все демонстрации по порядку с чистого листа; вывод сохраняется в results/.
cd "$(dirname "$0")/.."
rm -f results/*.txt results/*.csv
./scripts/reset.sh > results/00_setup.txt 2>&1 || { echo "setup failed"; exit 1; }
run() { echo "##### $1"; bash scripts/$1.sh > results/$2.txt 2>&1; tail -2 results/$2.txt; }
run demo_roles 01_roles
run demo_rls_audit 02_rls_audit
run demo_replication 03_replication
run backup_restore 04_backup_restore
run lag_test 05_lag
run analytics_on_replica 06_analytics_replica
run failure_replica 07_failure_replica
run failure_primary 08_failure_primary
echo "##### ВСЁ ГОТОВО"
