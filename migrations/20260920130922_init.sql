-- +goose Up
CREATE TABLE routes (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    number VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE stops (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    latitude NUMERIC(9, 6) NOT NULL,
    longitude NUMERIC(9, 6) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_stops_latitude
        CHECK (latitude BETWEEN -90 AND 90),

    CONSTRAINT chk_stops_longitude
        CHECK (longitude BETWEEN -180 AND 180)
);

CREATE TABLE route_stops (
    route_id BIGINT NOT NULL,
    stop_id BIGINT NOT NULL,
    sequence_no SMALLINT NOT NULL,

    PRIMARY KEY (route_id, stop_id),

    CONSTRAINT fk_route_stops_route
        FOREIGN KEY (route_id)
        REFERENCES routes(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_route_stops_stop
        FOREIGN KEY (stop_id)
        REFERENCES stops(id)
        ON DELETE RESTRICT,

    CONSTRAINT chk_route_stops_sequence
        CHECK (sequence_no > 0),

    CONSTRAINT uq_route_stops_sequence
        UNIQUE (route_id, sequence_no)
);

CREATE TABLE vehicles (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    fleet_number VARCHAR(30) NOT NULL UNIQUE,
    model VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_vehicles_status
        CHECK (
            status IN (
                'REGISTERED',
                'IN_SERVICE',
                'OUT_OF_SERVICE',
                'RETIRED'
            )
        )
);

CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    login VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_users_role
        CHECK (role IN ('ADMIN', 'DISPATCHER'))
);

CREATE TABLE trips (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    route_id BIGINT NOT NULL,
    vehicle_id BIGINT NOT NULL,
    planned_start TIMESTAMPTZ NOT NULL,
    planned_end TIMESTAMPTZ NOT NULL,
    actual_start TIMESTAMPTZ,
    actual_end TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_trips_route
        FOREIGN KEY (route_id)
        REFERENCES routes(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_trips_vehicle
        FOREIGN KEY (vehicle_id)
        REFERENCES vehicles(id)
        ON DELETE RESTRICT,

    CONSTRAINT chk_trips_status
        CHECK (
            status IN (
                'PLANNED',
                'IN_PROGRESS',
                'COMPLETED',
                'CANCELLED'
            )
        ),

    CONSTRAINT chk_trips_planned_time
        CHECK (planned_end > planned_start),

    CONSTRAINT chk_trips_actual_time
        CHECK (
            actual_end IS NULL
            OR actual_start IS NULL
            OR actual_end >= actual_start
        )
);

CREATE TABLE incidents (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    trip_id BIGINT NOT NULL,
    type VARCHAR(30) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'OPEN',
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ,
    resolved_by_user_id BIGINT,

    CONSTRAINT fk_incidents_trip
        FOREIGN KEY (trip_id)
        REFERENCES trips(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_incidents_resolved_by
        FOREIGN KEY (resolved_by_user_id)
        REFERENCES users(id)
        ON DELETE RESTRICT,

    CONSTRAINT chk_incidents_type
        CHECK (type IN ('DELAY', 'NO_TELEMETRY')),

    CONSTRAINT chk_incidents_status
        CHECK (status IN ('OPEN', 'RESOLVED')),

    CONSTRAINT chk_incidents_resolution
        CHECK (
            (
                status = 'OPEN'
                AND resolved_at IS NULL
                AND resolved_by_user_id IS NULL
            )
            OR
            (
                status = 'RESOLVED'
                AND resolved_at IS NOT NULL
                AND resolved_by_user_id IS NOT NULL
            )
        )
);

CREATE INDEX idx_trips_route_status
    ON trips(route_id, status);

CREATE INDEX idx_trips_vehicle_status
    ON trips(vehicle_id, status);

CREATE INDEX idx_incidents_trip_status
    ON incidents(trip_id, status);


-- +goose Down
SELECT 'down SQL query';
DROP TABLE IF EXISTS incidents; 
DROP TABLE IF EXISTS trips; 
DROP TABLE IF EXISTS users;  
DROP TABLE IF EXISTS vehicles; 
DROP TABLE IF EXISTS route_stops; 
DROP TABLE IF EXISTS stops; 
DROP TABLE IF EXISTS routes;