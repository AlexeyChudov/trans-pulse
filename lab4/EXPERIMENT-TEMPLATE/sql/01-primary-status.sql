\pset pager off

SELECT
  current_database() AS database_name,
  current_user AS connected_as,
  pg_is_in_recovery() AS is_in_recovery,
  pg_current_wal_lsn() AS current_wal_lsn;

SELECT
  application_name,
  client_addr,
  state,
  sync_state,
  sent_lsn,
  write_lsn,
  flush_lsn,
  replay_lsn
FROM pg_stat_replication
ORDER BY application_name;
