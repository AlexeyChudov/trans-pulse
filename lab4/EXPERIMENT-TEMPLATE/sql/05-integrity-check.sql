\pset pager off

SELECT
  count(*) AS probe_count,
  min(id) AS first_probe_id,
  max(id) AS last_probe_id
FROM app.replication_probe;

SELECT marker, count(*) AS duplicate_count
FROM app.replication_probe
GROUP BY marker
HAVING count(*) > 1;

SELECT
  constraint_name,
  constraint_type
FROM information_schema.table_constraints
WHERE table_schema = 'app'
  AND table_name = 'replication_probe'
ORDER BY constraint_name;
