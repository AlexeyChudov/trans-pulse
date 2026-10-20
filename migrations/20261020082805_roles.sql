-- +goose Up

-- admin
CREATE ROLE admin SUPERUSER;


--app group for accessing and managing data
CREATE ROLE app_rw NOLOGIN;

GRANT CONNECT ON DATABASE "trans-pulse_db" TO app_rw;
GRANT USAGE ON SCHEMA public TO app_rw;

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_rw;
-- GRANT USAGE, SELECT, UPDATE, DELETE, INSERT ON ALL INDEXES IN SCHEMA public TO app_rw;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_rw;

-- role for concrete user 
CREATE ROLE app_user LOGIN PASSWORD 'halaMadrid7';
GRANT app_rw TO app_user;


CREATE ROLE analytic NOLOGIN; 

GRANT CONNECT ON DATABASE "trans-pulse_db" TO analytic;
GRANT USAGE ON SCHEMA public TO analytic;

GRANT SELECT ON ALL TABLES IN SCHEMA public TO analytic;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO analytic;
-- GRANT USAGE, SELECT ON ALL INDEXES IN SCHEMA public TO analytic;

-- +goose Down

DELETE ROLE admin;
DELETE ROLE app_rw;
DELETE ROLE app_user;
DELETE ROLE analytic;