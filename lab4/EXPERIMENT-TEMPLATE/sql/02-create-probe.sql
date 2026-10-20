\pset pager off

INSERT INTO app.replication_probe (marker)
VALUES (
  'probe-'
  || to_char(clock_timestamp(), 'YYYYMMDD-HH24MISS-MS')
  || '-'
  || pg_backend_pid()
)
RETURNING id, marker, created_at;

SELECT pg_current_wal_lsn() AS primary_wal_lsn;
