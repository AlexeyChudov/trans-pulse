package mongorepo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

// Коллекции документной части.
const (
	CollTelemetry     = "telemetry_buckets"
	CollJournals      = "trip_journals"
	CollCases         = "incident_cases"
	CollStopPoints    = "stop_points"
	CollRouteProfiles = "route_profiles"
)

// Схемы валидации ($jsonSchema). Применяются при старте приложения (EnsureSchema) с validationLevel=strict
// и validationAction=error: документ, нарушающий схему, не записывается.

var (
	integer  = bson.M{"bsonType": bson.A{"int", "long"}}
	dateType = bson.M{"bsonType": "date"}
	version1 = bson.M{"bsonType": bson.A{"int", "long"}, "enum": bson.A{1}}
)

func number(min, max float64) bson.M {
	return bson.M{"bsonType": "number", "minimum": min, "maximum": max}
}

func text(min, max int) bson.M {
	return bson.M{"bsonType": "string", "minLength": min, "maxLength": max}
}

func objectSchema(required bson.A, props bson.M) bson.M {
	props["_id"] = bson.M{} // _id разрешён всегда
	return bson.M{"bsonType": "object", "required": required, "additionalProperties": false, "properties": props}
}

func telemetrySchema() bson.M {
	point := bson.M{
		"bsonType": "object", "required": bson.A{"ts", "lat", "lon", "speed"}, "additionalProperties": false,
		"properties": bson.M{
			"ts": dateType, "lat": number(-90, 90), "lon": number(-180, 180), "speed": number(0, 250), "trip_id": integer,
		},
	}
	return objectSchema(bson.A{"vehicle_id", "hour", "count", "points", "schema_version"}, bson.M{
		"vehicle_id":     integer,
		"hour":           dateType,
		"count":          bson.M{"bsonType": bson.A{"int", "long"}, "minimum": 0, "maximum": domain.MaxBucketPoints},
		"points":         bson.M{"bsonType": "array", "maxItems": domain.MaxBucketPoints, "items": point},
		"schema_version": version1,
	})
}

func journalSchema() bson.M {
	branch := func(types bson.A, required bson.A) bson.M {
		b := bson.M{"properties": bson.M{"type": bson.M{"enum": types}}}
		if len(required) > 0 {
			b["required"] = required
		}
		return b
	}
	event := bson.M{
		"bsonType": "object", "required": bson.A{"at", "type"}, "additionalProperties": false,
		"properties": bson.M{
			"at":            dateType,
			"type":          bson.M{"bsonType": "string"},
			"stop_id":       integer,
			"delay_seconds": bson.M{"bsonType": bson.A{"int", "long"}, "minimum": 0, "maximum": 86400},
			"note":          text(0, 500),
		},
		// Правила, зависящие от типа события: ровно одна из веток должна подойти.
		"oneOf": bson.A{
			branch(bson.A{"DELAY_DETECTED"}, bson.A{"delay_seconds"}),
			branch(bson.A{"ARRIVED_AT_STOP"}, bson.A{"stop_id"}),
			branch(bson.A{"DEPARTED", "TELEMETRY_LOST", "TELEMETRY_RESTORED", "NOTE"}, nil),
		},
	}
	return objectSchema(bson.A{"events", "last_event_at", "created_at", "updated_at", "schema_version"}, bson.M{
		"events":         bson.M{"bsonType": "array", "maxItems": domain.MaxJournalEvents, "items": event},
		"last_event_at":  bson.M{"bsonType": bson.A{"date", "null"}},
		"created_at":     dateType,
		"updated_at":     dateType,
		"schema_version": version1,
	})
}

func caseSchema() bson.M {
	comment := bson.M{
		"bsonType": "object", "required": bson.A{"author_id", "text", "at"}, "additionalProperties": false,
		"properties": bson.M{"author_id": integer, "text": text(1, 2000), "at": dateType},
	}
	s := objectSchema(bson.A{"trip_id", "vehicle_id", "route_id", "title", "tags", "status", "comments", "created_by", "created_at", "updated_at", "schema_version"}, bson.M{
		"trip_id":        integer,
		"vehicle_id":     integer,
		"route_id":       integer,
		"title":          text(3, 200),
		"tags":           bson.M{"bsonType": "array", "maxItems": 10, "uniqueItems": true, "items": text(1, 30)},
		"status":         bson.M{"bsonType": "string"},
		"comments":       bson.M{"bsonType": "array", "maxItems": domain.MaxCaseComments, "items": comment},
		"created_by":     integer,
		"created_at":     dateType,
		"updated_at":     dateType,
		"closed_at":      dateType,
		"closed_by":      integer,
		"schema_version": version1,
	})
	// Правило: закрытая карточка обязана иметь closed_at и closed_by, открытая — не иметь их.
	s["oneOf"] = bson.A{
		bson.M{"properties": bson.M{"status": bson.M{"enum": bson.A{"OPEN"}}},
			"not": bson.M{"anyOf": bson.A{bson.M{"required": bson.A{"closed_at"}}, bson.M{"required": bson.A{"closed_by"}}}}},
		bson.M{"properties": bson.M{"status": bson.M{"enum": bson.A{"CLOSED"}}}, "required": bson.A{"closed_at", "closed_by"}},
	}
	return s
}

func stopPointSchema() bson.M {
	location := bson.M{
		"bsonType": "object", "required": bson.A{"type", "coordinates"}, "additionalProperties": false,
		"properties": bson.M{
			"type": bson.M{"enum": bson.A{"Point"}},
			// GeoJSON: [долгота, широта]
			"coordinates": bson.M{"bsonType": "array", "minItems": 2, "maxItems": 2, "items": bson.A{number(-180, 180), number(-90, 90)}},
		},
	}
	return objectSchema(bson.A{"name", "location", "route_ids", "updated_at", "schema_version"}, bson.M{
		"name":      text(1, 150),
		"location":  location,
		"route_ids": bson.M{"bsonType": "array", "uniqueItems": true, "items": integer},
		"attributes": bson.M{
			"bsonType": "object", "maxProperties": 20,
			"additionalProperties": bson.M{"bsonType": bson.A{"string", "bool", "int", "long", "double"}},
		},
		"updated_at":     dateType,
		"schema_version": version1,
	})
}

func routeProfileSchema() bson.M {
	ref := bson.M{
		"bsonType": "object", "required": bson.A{"stop_id", "seq"}, "additionalProperties": false,
		"properties": bson.M{"stop_id": integer, "seq": bson.M{"bsonType": bson.A{"int", "long"}, "minimum": 1}},
	}
	return objectSchema(bson.A{"stops", "updated_at", "schema_version"}, bson.M{
		"stops":          bson.M{"bsonType": "array", "maxItems": domain.MaxRouteStops, "items": ref},
		"updated_at":     dateType,
		"schema_version": version1,
	})
}

type collectionSpec struct {
	name    string
	schema  bson.M
	indexes []mongo.IndexModel
}

func specs() []collectionSpec {
	return []collectionSpec{
		{CollTelemetry, telemetrySchema(), []mongo.IndexModel{
			// один документ на автобус и час: именно этот индекс делает upsert корзины атомарным
			{Keys: bson.D{{Key: "vehicle_id", Value: 1}, {Key: "hour", Value: 1}}, Options: options.Index().SetUnique(true)},
		}},
		{CollJournals, journalSchema(), nil}, // _id = номер рейса
		{CollCases, caseSchema(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "trip_id", Value: 1}}},
			{Keys: bson.D{{Key: "vehicle_id", Value: 1}, {Key: "status", Value: 1}}},
		}},
		{CollStopPoints, stopPointSchema(), []mongo.IndexModel{
			{Keys: bson.D{{Key: "location", Value: "2dsphere"}}}, // геопоиск ближайших остановок
			{Keys: bson.D{{Key: "route_ids", Value: 1}}},         // какие остановки входят в маршрут
		}},
		{CollRouteProfiles, routeProfileSchema(), nil},
	}
}

// EnsureSchema создаёт коллекции с валидацией и индексами; для существующих коллекций обновляет валидатор. Безопасно вызывать при каждом старте.
func EnsureSchema(ctx context.Context, db *mongo.Database) error {
	for _, s := range specs() {
		names, err := db.ListCollectionNames(ctx, bson.D{{Key: "name", Value: s.name}})
		if err != nil {
			return fmt.Errorf("mongo: список коллекций: %w", err)
		}
		validator := bson.M{"$jsonSchema": s.schema}
		if len(names) == 0 {
			opts := options.CreateCollection().SetValidator(validator).SetValidationLevel("strict").SetValidationAction("error")
			err = db.CreateCollection(ctx, s.name, opts)
		} else {
			err = db.RunCommand(ctx, bson.D{
				{Key: "collMod", Value: s.name}, {Key: "validator", Value: validator},
				{Key: "validationLevel", Value: "strict"}, {Key: "validationAction", Value: "error"},
			}).Err()
		}
		if err != nil {
			return fmt.Errorf("mongo: коллекция %s: %w", s.name, err)
		}
		if len(s.indexes) > 0 {
			if _, err := db.Collection(s.name).Indexes().CreateMany(ctx, s.indexes); err != nil {
				return fmt.Errorf("mongo: индексы %s: %w", s.name, err)
			}
		}
	}
	return nil
}
