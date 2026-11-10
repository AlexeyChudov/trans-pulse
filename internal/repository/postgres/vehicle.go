package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

type VehicleRepository struct{ pool *pgxpool.Pool }

func NewVehicleRepository(pool *pgxpool.Pool) *VehicleRepository {
	return &VehicleRepository{pool: pool}
}

const vehicleCols = "id, fleet_number, model, status, created_at"

func scanVehicle(row pgx.CollectableRow) (domain.Vehicle, error) {
	var v domain.Vehicle
	var status string
	err := row.Scan(&v.ID, &v.FleetNumber, &v.Model, &status, &v.CreatedAt)
	v.Status = domain.VehicleStatus(status)
	return v, err
}

func (r *VehicleRepository) List(ctx context.Context) ([]domain.Vehicle, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+vehicleCols+" FROM vehicles ORDER BY id")
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanVehicle)
}

func (r *VehicleRepository) Get(ctx context.Context, id int64) (domain.Vehicle, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+vehicleCols+" FROM vehicles WHERE id = $1", id)
	if err != nil {
		return domain.Vehicle{}, err
	}
	v, err := pgx.CollectOneRow(rows, scanVehicle)
	return v, mapErr(err, domain.ErrInvalidInput)
}

func (r *VehicleRepository) Create(ctx context.Context, v *domain.Vehicle) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO vehicles (fleet_number, model, status) VALUES ($1, $2, $3) RETURNING id, created_at`,
		v.FleetNumber, v.Model, string(v.Status)).Scan(&v.ID, &v.CreatedAt)
	return mapErr(err, domain.ErrInvalidInput)
}

func (r *VehicleRepository) UpdateStatus(ctx context.Context, id int64, status domain.VehicleStatus) error {
	tag, err := r.pool.Exec(ctx, `UPDATE vehicles SET status = $2 WHERE id = $1`, id, string(status))
	if err != nil {
		return mapErr(err, domain.ErrInvalidInput)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
