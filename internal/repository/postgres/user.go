package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

type UserRepository struct{ pool *pgxpool.Pool }

func NewUserRepository(pool *pgxpool.Pool) *UserRepository { return &UserRepository{pool: pool} }

const userCols = "id, login, password_hash, role, is_active, created_at"

func scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	var role string
	err := row.Scan(&u.ID, &u.Login, &u.PasswordHash, &role, &u.IsActive, &u.CreatedAt)
	u.Role = domain.Role(role)
	return u, err
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (login, password_hash, role, is_active) VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		u.Login, u.PasswordHash, string(u.Role), u.IsActive).Scan(&u.ID, &u.CreatedAt)
	return mapErr(err, domain.ErrInvalidInput)
}

func (r *UserRepository) GetByLogin(ctx context.Context, login string) (domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx, "SELECT "+userCols+" FROM users WHERE login = $1", login))
	return u, mapErr(err, domain.ErrInvalidInput)
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx, "SELECT "+userCols+" FROM users WHERE id = $1", id))
	return u, mapErr(err, domain.ErrInvalidInput)
}
