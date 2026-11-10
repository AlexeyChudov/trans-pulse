package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

type TripRepository struct{ pool *pgxpool.Pool }

func NewTripRepository(pool *pgxpool.Pool) *TripRepository { return &TripRepository{pool: pool} }

const tripCols = "id, route_id, vehicle_id, planned_start, planned_end, actual_start, actual_end, status, created_at"

func scanTrip(row pgx.CollectableRow) (domain.Trip, error) {
	var t domain.Trip
	var status string
	err := row.Scan(&t.ID, &t.RouteID, &t.VehicleID, &t.PlannedStart, &t.PlannedEnd, &t.ActualStart, &t.ActualEnd, &status, &t.CreatedAt)
	t.Status = domain.TripStatus(status)
	return t, err
}

func (r *TripRepository) List(ctx context.Context, f domain.TripFilter) ([]domain.Trip, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+tripCols+` FROM trips
		 WHERE ($1::text = '' OR status = $1::text) AND ($2::bigint = 0 OR route_id = $2::bigint)
		 ORDER BY id DESC LIMIT $3 OFFSET $4`,
		string(f.Status), f.RouteID, f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanTrip)
}

func (r *TripRepository) Get(ctx context.Context, id int64) (domain.Trip, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+tripCols+" FROM trips WHERE id = $1", id)
	if err != nil {
		return domain.Trip{}, err
	}
	t, err := pgx.CollectOneRow(rows, scanTrip)
	return t, mapErr(err, domain.ErrInvalidInput)
}

func (r *TripRepository) Create(ctx context.Context, t *domain.Trip) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO trips (route_id, vehicle_id, planned_start, planned_end, status) VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		t.RouteID, t.VehicleID, t.PlannedStart, t.PlannedEnd, string(t.Status)).Scan(&t.ID, &t.CreatedAt)
	return mapErr(err, domain.ErrInvalidInput)
}

// Transition — один атомарный UPDATE: рейс переходит в новый статус, только если сейчас он в статусе from.
// Правило «один активный рейс на автобус» гарантирует частичный уникальный индекс (нарушение → ErrConflict).
func (r *TripRepository) Transition(ctx context.Context, id int64, from, to domain.TripStatus, at time.Time) (domain.Trip, error) {
	rows, err := r.pool.Query(ctx,
		`UPDATE trips SET status = $3::varchar,
		        actual_start = CASE WHEN $3::varchar = 'IN_PROGRESS' THEN $4::timestamptz ELSE actual_start END,
		        actual_end   = CASE WHEN $3::varchar = 'COMPLETED'   THEN $4::timestamptz ELSE actual_end END
		 WHERE id = $1 AND status = $2::varchar
		 RETURNING `+tripCols,
		id, string(from), string(to), at)
	if err != nil {
		return domain.Trip{}, mapErr(err, domain.ErrInvalidInput)
	}
	t, err := pgx.CollectOneRow(rows, scanTrip)
	if err == pgx.ErrNoRows {
		return domain.Trip{}, notFoundOrConflict(ctx, r.pool, "trips", id, "статус рейса уже изменился")
	}
	return t, mapErr(err, domain.ErrInvalidInput)
}
