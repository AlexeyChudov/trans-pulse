SELECT md5(string_agg(concat_ws('|', route_id, vehicle_id, planned_start, actual_start, actual_end, status), ',' ORDER BY id)) AS trips_md5,
       (SELECT md5(string_agg(concat_ws('|', trip_id, type, status, created_at, resolved_at, resolved_by_user_id), ',' ORDER BY id)) FROM incidents) AS incidents_md5
FROM trips;
