package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

type RouteRepository struct{ pool *pgxpool.Pool }

func NewRouteRepository(pool *pgxpool.Pool) *RouteRepository { return &RouteRepository{pool: pool} }

const routeCols = "id, number, name, is_active, created_at"

func scanRoute(row pgx.CollectableRow) (domain.Route, error) {
	var r domain.Route
	err := row.Scan(&r.ID, &r.Number, &r.Name, &r.IsActive, &r.CreatedAt)
	return r, err
}

func (r *RouteRepository) List(ctx context.Context, onlyActive bool) ([]domain.Route, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+routeCols+" FROM routes WHERE ($1 = FALSE OR is_active) ORDER BY id", onlyActive)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanRoute)
}

func (r *RouteRepository) Get(ctx context.Context, id int64) (domain.Route, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+routeCols+" FROM routes WHERE id = $1", id)
	if err != nil {
		return domain.Route{}, err
	}
	route, err := pgx.CollectOneRow(rows, scanRoute)
	return route, mapErr(err, domain.ErrInvalidInput)
}

func (r *RouteRepository) Create(ctx context.Context, route *domain.Route) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO routes (number, name, is_active) VALUES ($1, $2, $3) RETURNING id, created_at`,
		route.Number, route.Name, route.IsActive).Scan(&route.ID, &route.CreatedAt)
	return mapErr(err, domain.ErrInvalidInput)
}

func (r *RouteRepository) Update(ctx context.Context, route *domain.Route) error {
	tag, err := r.pool.Exec(ctx, `UPDATE routes SET number = $2, name = $3, is_active = $4 WHERE id = $1`,
		route.ID, route.Number, route.Name, route.IsActive)
	if err != nil {
		return mapErr(err, domain.ErrInvalidInput)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete: если на маршрут ссылаются рейсы или остановки, ограничение ON DELETE RESTRICT даёт ErrConflict.
func (r *RouteRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM routes WHERE id = $1`, id)
	if err != nil {
		return mapErr(err, domain.ErrConflict)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

const stopCols = "s.id, s.name, s.latitude::float8, s.longitude::float8, s.is_active, s.created_at"

func (r *RouteRepository) ListStops(ctx context.Context, routeID int64) ([]domain.RouteStop, error) {
	rows, err := r.pool.Query(ctx,
		"SELECT "+stopCols+", rs.sequence_no FROM route_stops rs JOIN stops s ON s.id = rs.stop_id WHERE rs.route_id = $1 ORDER BY rs.sequence_no",
		routeID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.RouteStop, error) {
		var rs domain.RouteStop
		err := row.Scan(&rs.ID, &rs.Name, &rs.Latitude, &rs.Longitude, &rs.IsActive, &rs.CreatedAt, &rs.SequenceNo)
		return rs, err
	})
}

func (r *RouteRepository) GetStop(ctx context.Context, id int64) (domain.Stop, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+stopCols+" FROM stops s WHERE s.id = $1", id)
	if err != nil {
		return domain.Stop{}, err
	}
	stop, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (domain.Stop, error) {
		var s domain.Stop
		err := row.Scan(&s.ID, &s.Name, &s.Latitude, &s.Longitude, &s.IsActive, &s.CreatedAt)
		return s, err
	})
	return stop, mapErr(err, domain.ErrInvalidInput)
}

// ListAllStops возвращает активные остановки.
func (r *RouteRepository) ListAllStops(ctx context.Context) ([]domain.Stop, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+stopCols+" FROM stops s WHERE s.is_active ORDER BY s.id")
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Stop, error) {
		var s domain.Stop
		err := row.Scan(&s.ID, &s.Name, &s.Latitude, &s.Longitude, &s.IsActive, &s.CreatedAt)
		return s, err
	})
}
