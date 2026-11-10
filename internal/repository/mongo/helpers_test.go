package mongorepo_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	mongorepo "github.com/AlexeyChudov/trans-pulse/internal/repository/mongo"
)

var ctx = context.Background()

// Интеграционные тесты идут против настоящей MongoDB; без переменной они пропускаются.
const envURI = "API_TEST_MONGO_URI"

func testDB(t *testing.T) *mongo.Database {
	t.Helper()
	uri := os.Getenv(envURI)
	if uri == "" {
		t.Skipf("%s не задан: интеграционные тесты MongoDB пропущены", envURI)
	}
	client, err := mongorepo.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("подключение к MongoDB: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(ctx) })
	db := client.Database("trans_pulse_test")
	if err := db.Drop(ctx); err != nil {
		t.Fatalf("очистка БД: %v", err)
	}
	if err := mongorepo.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return db
}

// insertRaw вставляет документ в обход репозиториев — так проверяются правила схемы валидации.
func insertRaw(db *mongo.Database, coll string, doc bson.M) error {
	_, err := db.Collection(coll).InsertOne(ctx, doc)
	return err
}

func isValidationFailure(err error) bool {
	var we mongo.WriteException
	if errors.As(err, &we) {
		for _, e := range we.WriteErrors {
			if e.Code == 121 { // DocumentValidationFailure
				return true
			}
		}
	}
	return false
}

var base = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
