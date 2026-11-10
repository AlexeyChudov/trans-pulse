package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

type IncidentRepository struct{ pool *pgxpool.Pool }

func NewIncidentRepository(pool *pgxpool.Pool) *IncidentRepository {
	return &IncidentRepository{pool: pool}
}

const incidentCols = "id, trip_id, type, status, description, created_at, resolved_at, resolved_by_user_id"

func scanIncident(row pgx.CollectableRow) (domain.Incident, error) {
	var i domain.Incident
	var status string
	err := row.Scan(&i.ID, &i.TripID, &i.Type, &status, &i.Description, &i.CreatedAt, &i.ResolvedAt, &i.ResolvedByUserID)
	i.Status = domain.IncidentStatus(status)
	return i, err
}

func (r *IncidentRepository) List(ctx context.Context, f domain.IncidentFilter) ([]domain.Incident, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+incidentCols+` FROM incidents
		 WHERE ($1::text = '' OR status = $1::text) AND ($2::bigint = 0 OR trip_id = $2::bigint)
		 ORDER BY id DESC LIMIT $3 OFFSET $4`,
		string(f.Status), f.TripID, f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanIncident)
}

func (r *IncidentRepository) Get(ctx context.Context, id int64) (domain.Incident, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+incidentCols+" FROM incidents WHERE id = $1", id)
	if err != nil {
		return domain.Incident{}, err
	}
	i, err := pgx.CollectOneRow(rows, scanIncident)
	return i, mapErr(err, domain.ErrInvalidInput)
}

// Resolve закрывает только открытый инцидент (атомарный UPDATE с проверкой статуса).
func (r *IncidentRepository) Resolve(ctx context.Context, id, userID int64, at time.Time) (domain.Incident, error) {
	rows, err := r.pool.Query(ctx,
		`UPDATE incidents SET status = 'RESOLVED', resolved_at = $3, resolved_by_user_id = $2
		 WHERE id = $1 AND status = 'OPEN' RETURNING `+incidentCols,
		id, userID, at)
	if err != nil {
		return domain.Incident{}, mapErr(err, domain.ErrInvalidInput)
	}
	i, err := pgx.CollectOneRow(rows, scanIncident)
	if err == pgx.ErrNoRows {
		return domain.Incident{}, notFoundOrConflict(ctx, r.pool, "incidents", id, "инцидент уже закрыт")
	}
	return i, mapErr(err, domain.ErrInvalidInput)
}
