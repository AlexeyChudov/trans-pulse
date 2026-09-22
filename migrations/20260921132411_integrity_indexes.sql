-- +goose Up

CREATE UNIQUE INDEX uq_trips_one_active_per_vehicle
    ON trips(vehicle_id)
    WHERE status = 'IN_PROGRESS';

CREATE UNIQUE INDEX uq_incidents_one_open_per_type
    ON incidents(trip_id, type)
    WHERE status = 'OPEN';


-- +goose Down

DROP INDEX IF EXISTS uq_incidents_one_open_per_type;
DROP INDEX IF EXISTS uq_trips_one_active_per_vehicle;