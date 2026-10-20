\pset pager off

SELECT id, marker, created_at
FROM app.replication_probe
ORDER BY id DESC
LIMIT 20;
