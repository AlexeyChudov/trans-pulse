// Package postgres реализует интерфейсы репозиториев из domain на чистом pgx (без database/sql).
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

// Коды ошибок PostgreSQL, которые переводятся в ошибки домена.
const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
	codeCheckViolation      = "23514"
)

func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: подключение: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}

// mapErr переводит ошибки БД в ошибки домена. foreignKey — чем считать нарушение внешнего ключа:
// при вставке это неверные данные (ErrInvalidInput), при удалении — конфликт (ErrConflict).
func mapErr(err error, foreignKey error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case codeUniqueViolation:
			return fmt.Errorf("%w: нарушено ограничение %s", domain.ErrConflict, pgErr.ConstraintName)
		case codeForeignKeyViolation:
			return fmt.Errorf("%w: нарушена связь %s", foreignKey, pgErr.ConstraintName)
		case codeCheckViolation:
			return fmt.Errorf("%w: нарушено ограничение %s", domain.ErrInvalidInput, pgErr.ConstraintName)
		}
	}
	return err
}

// exists сообщает, есть ли строка в таблице (имя таблицы — только из констант пакета).
func exists(ctx context.Context, pool *pgxpool.Pool, table string, id int64) (bool, error) {
	var ok bool
	err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+table+" WHERE id = $1)", id).Scan(&ok)
	return ok, err
}

// notFoundOrConflict различает «строки нет» и «строка есть, но не в нужном состоянии».
func notFoundOrConflict(ctx context.Context, pool *pgxpool.Pool, table string, id int64, conflictMsg string) error {
	ok, err := exists(ctx, pool, table, id)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrNotFound
	}
	return fmt.Errorf("%w: %s", domain.ErrConflict, conflictMsg)
}
