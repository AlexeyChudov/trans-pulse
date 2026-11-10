// Package mongorepo реализует репозитории документов (domain.*Repository) на MongoDB.
// Правила, которые должны выполняться атомарно, закодированы в фильтрах операций записи;
// схема валидации коллекций (schema.go) — последний рубеж защиты данных.
package mongorepo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

const (
	codeDocumentValidationFailure = 121
	schemaVersion                 = int32(1)
)

func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo: подключение: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("mongo: ping: %w", err)
	}
	return client, nil
}

// mapErr переводит ошибки MongoDB в ошибки домена.
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, mongo.ErrNoDocuments):
		return domain.ErrNotFound
	case mongo.IsDuplicateKeyError(err):
		return fmt.Errorf("%w: запись уже существует", domain.ErrConflict)
	case isValidationFailure(err):
		return fmt.Errorf("%w: документ не прошёл проверку схемы MongoDB", domain.ErrInvalidInput)
	}
	return err
}

func isValidationFailure(err error) bool {
	var we mongo.WriteException
	if errors.As(err, &we) {
		for _, e := range we.WriteErrors {
			if e.Code == codeDocumentValidationFailure {
				return true
			}
		}
	}
	var ce mongo.CommandError
	return errors.As(err, &ce) && ce.Code == codeDocumentValidationFailure
}
