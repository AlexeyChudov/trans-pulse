package mongorepo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

// TelemetryRepository хранит телеметрию «корзинами» (bucket pattern): один документ на автобус и час,
// точки встроены в массив. Так в коллекции в десятки раз меньше документов, а история автобуса читается диапазоном.
type TelemetryRepository struct{ coll *mongo.Collection }

func NewTelemetryRepository(db *mongo.Database) *TelemetryRepository {
	return &TelemetryRepository{coll: db.Collection(CollTelemetry)}
}

type pointDoc struct {
	TS     time.Time `bson:"ts"`
	Lat    float64   `bson:"lat"`
	Lon    float64   `bson:"lon"`
	Speed  float64   `bson:"speed"`
	TripID int64     `bson:"trip_id,omitempty"`
}

// Append добавляет точку одним атомарным upsert: если корзина часа уже заполнена, фильтр не совпадает,
// upsert пытается создать дубликат ключа (vehicle_id, hour) и возвращает ErrConflict.
func (r *TelemetryRepository) Append(ctx context.Context, p domain.TelemetryPoint) error {
	hour := p.Timestamp.UTC().Truncate(time.Hour)
	filter := bson.D{
		{Key: "vehicle_id", Value: p.VehicleID},
		{Key: "hour", Value: hour},
		{Key: "count", Value: bson.D{{Key: "$lt", Value: domain.MaxBucketPoints}}},
	}
	update := bson.D{
		{Key: "$push", Value: bson.D{{Key: "points", Value: pointDoc{TS: p.Timestamp.UTC(), Lat: p.Latitude, Lon: p.Longitude, Speed: p.SpeedKmh, TripID: p.TripID}}}},
		{Key: "$inc", Value: bson.D{{Key: "count", Value: 1}}},
		{Key: "$setOnInsert", Value: bson.D{{Key: "schema_version", Value: schemaVersion}}},
	}
	_, err := r.coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	return mapErr(err)
}

func (r *TelemetryRepository) History(ctx context.Context, vehicleID int64, from, to time.Time, limit int) ([]domain.TelemetryPoint, error) {
	pipeline := mongo.Pipeline{
		// сначала отбираем корзины по индексу (vehicle_id, hour), затем разворачиваем только нужные
		{{Key: "$match", Value: bson.D{
			{Key: "vehicle_id", Value: vehicleID},
			{Key: "hour", Value: bson.D{{Key: "$gte", Value: from.UTC().Truncate(time.Hour)}, {Key: "$lt", Value: to.UTC()}}},
		}}},
		{{Key: "$unwind", Value: "$points"}},
		{{Key: "$match", Value: bson.D{{Key: "points.ts", Value: bson.D{{Key: "$gte", Value: from.UTC()}, {Key: "$lt", Value: to.UTC()}}}}}},
		{{Key: "$sort", Value: bson.D{{Key: "points.ts", Value: 1}}}},
		{{Key: "$limit", Value: int64(limit)}},
		{{Key: "$project", Value: bson.D{{Key: "_id", Value: 0}, {Key: "vehicle_id", Value: 1}, {Key: "point", Value: "$points"}}}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, mapErr(err)
	}
	var rows []struct {
		VehicleID int64    `bson:"vehicle_id"`
		Point     pointDoc `bson:"point"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.TelemetryPoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.TelemetryPoint{VehicleID: row.VehicleID, TripID: row.Point.TripID, Timestamp: row.Point.TS.UTC(),
			Latitude: row.Point.Lat, Longitude: row.Point.Lon, SpeedKmh: row.Point.Speed})
	}
	return out, nil
}
