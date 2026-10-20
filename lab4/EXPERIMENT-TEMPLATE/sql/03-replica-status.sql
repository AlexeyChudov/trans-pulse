\pset pager off

SELECT
  current_database() AS database_name,
  current_user AS connected_as,
  pg_is_in_recovery() AS is_in_recovery,
  pg_last_wal_receive_lsn() AS last_received_lsn,
  pg_last_wal_replay_lsn() AS last_replayed_lsn,
  clock_timestamp() - pg_last_xact_replay_timestamp() AS time_since_last_replayed_transaction;

SELECT
  status,
  sender_host,
  sender_port,
  latest_end_lsn,
  latest_end_time
FROM pg_stat_wal_receiver;
