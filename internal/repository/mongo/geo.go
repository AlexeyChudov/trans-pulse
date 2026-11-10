package mongorepo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

// StopPointRepository: документная проекция остановки с GeoJSON-точкой (геопоиск) и обратными ссылками на маршруты.
type StopPointRepository struct{ coll *mongo.Collection }

func NewStopPointRepository(db *mongo.Database) *StopPointRepository {
	return &StopPointRepository{coll: db.Collection(CollStopPoints)}
}

type geoPoint struct {
	Type        string    `bson:"type"`
	Coordinates []float64 `bson:"coordinates"` // [долгота, широта]
}

type stopDoc struct {
	ID            int64          `bson:"_id"`
	Name          string         `bson:"name"`
	Location      geoPoint       `bson:"location"`
	RouteIDs      []int64        `bson:"route_ids"`
	Attributes    map[string]any `bson:"attributes,omitempty"`
	UpdatedAt     time.Time      `bson:"updated_at"`
	SchemaVersion int32          `bson:"schema_version"`
}

func (d stopDoc) toDomain() domain.StopPoint {
	s := domain.StopPoint{StopID: d.ID, Name: d.Name, RouteIDs: append([]int64{}, d.RouteIDs...), Attributes: d.Attributes, UpdatedAt: d.UpdatedAt.UTC()}
	if len(d.Location.Coordinates) == 2 {
		s.Longitude, s.Latitude = d.Location.Coordinates[0], d.Location.Coordinates[1]
	}
	if s.Attributes == nil {
		s.Attributes = map[string]any{}
	}
	return s
}

func (r *StopPointRepository) Create(ctx context.Context, s *domain.StopPoint) error {
	now := time.Now().UTC()
	doc := stopDoc{ID: s.StopID, Name: s.Name, Location: geoPoint{Type: "Point", Coordinates: []float64{s.Longitude, s.Latitude}},
		RouteIDs: []int64{}, Attributes: s.Attributes, UpdatedAt: now, SchemaVersion: schemaVersion}
	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		return mapErr(err)
	}
	s.UpdatedAt = now
	return nil
}

func (r *StopPointRepository) Get(ctx context.Context, stopID int64) (domain.StopPoint, error) {
	var doc stopDoc
	if err := r.coll.FindOne(ctx, bson.D{{Key: "_id", Value: stopID}}).Decode(&doc); err != nil {
		return domain.StopPoint{}, mapErr(err)
	}
	return doc.toDomain(), nil
}

func (r *StopPointRepository) SetAttributes(ctx context.Context, stopID int64, attrs map[string]any, at time.Time) (domain.StopPoint, error) {
	if attrs == nil {
		attrs = map[string]any{}
	}
	var doc stopDoc
	err := r.coll.FindOneAndUpdate(ctx, bson.D{{Key: "_id", Value: stopID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "attributes", Value: attrs}, {Key: "updated_at", Value: at.UTC()}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if err != nil {
		return domain.StopPoint{}, mapErr(err)
	}
	return doc.toDomain(), nil
}

// Delete удаляет проекцию, только если остановка не входит ни в один маршрут (целостность связи M:N).
func (r *StopPointRepository) Delete(ctx context.Context, stopID int64) error {
	res, err := r.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: stopID}, {Key: "route_ids", Value: bson.D{{Key: "$size", Value: 0}}}})
	if err != nil {
		return mapErr(err)
	}
	if res.DeletedCount > 0 {
		return nil
	}
	if _, err := r.Get(ctx, stopID); err != nil {
		return err
	}
	return fmt.Errorf("%w: остановка входит в маршруты", domain.ErrConflict)
}

func (r *StopPointRepository) Near(ctx context.Context, lat, lon float64, radiusM int, limit int) ([]domain.NearStop, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$geoNear", Value: bson.D{
			{Key: "near", Value: bson.D{{Key: "type", Value: "Point"}, {Key: "coordinates", Value: bson.A{lon, lat}}}},
			{Key: "distanceField", Value: "distance_m"},
			{Key: "maxDistance", Value: radiusM},
			{Key: "spherical", Value: true},
		}}},
		{{Key: "$limit", Value: int64(limit)}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, mapErr(err)
	}
	var rows []struct {
		Doc       stopDoc `bson:",inline"`
		DistanceM float64 `bson:"distance_m"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.NearStop, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.NearStop{StopPoint: row.Doc.toDomain(), DistanceM: row.DistanceM})
	}
	return out, nil
}

func (r *StopPointRepository) AddRoute(ctx context.Context, stopID, routeID int64) error {
	return r.touchRoute(ctx, stopID, "$addToSet", routeID)
}

func (r *StopPointRepository) RemoveRoute(ctx context.Context, stopID, routeID int64) error {
	return r.touchRoute(ctx, stopID, "$pull", routeID)
}

func (r *StopPointRepository) touchRoute(ctx context.Context, stopID int64, op string, routeID int64) error {
	res, err := r.coll.UpdateOne(ctx, bson.D{{Key: "_id", Value: stopID}}, bson.D{{Key: op, Value: bson.D{{Key: "route_ids", Value: routeID}}}})
	if err != nil {
		return mapErr(err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// RouteProfileRepository: состав маршрута — упорядоченные ссылки на остановки.
type RouteProfileRepository struct{ coll *mongo.Collection }

func NewRouteProfileRepository(db *mongo.Database) *RouteProfileRepository {
	return &RouteProfileRepository{coll: db.Collection(CollRouteProfiles)}
}

type stopRefDoc struct {
	StopID int64 `bson:"stop_id"`
	Seq    int   `bson:"seq"`
}

type profileDoc struct {
	ID            int64        `bson:"_id"`
	Stops         []stopRefDoc `bson:"stops"`
	UpdatedAt     time.Time    `bson:"updated_at"`
	SchemaVersion int32        `bson:"schema_version"`
}

func (d profileDoc) toDomain() domain.RouteProfile {
	p := domain.RouteProfile{RouteID: d.ID, Stops: make([]domain.RouteStopRef, 0, len(d.Stops)), UpdatedAt: d.UpdatedAt.UTC()}
	for _, s := range d.Stops {
		p.Stops = append(p.Stops, domain.RouteStopRef{StopID: s.StopID, Seq: s.Seq})
	}
	return p
}

func (r *RouteProfileRepository) Get(ctx context.Context, routeID int64) (domain.RouteProfile, error) {
	var doc profileDoc
	if err := r.coll.FindOne(ctx, bson.D{{Key: "_id", Value: routeID}}).Decode(&doc); err != nil {
		return domain.RouteProfile{}, mapErr(err)
	}
	return doc.toDomain(), nil
}

func (r *RouteProfileRepository) Replace(ctx context.Context, p *domain.RouteProfile) (*domain.RouteProfile, error) {
	doc := profileDoc{ID: p.RouteID, Stops: make([]stopRefDoc, 0, len(p.Stops)), UpdatedAt: time.Now().UTC(), SchemaVersion: schemaVersion}
	for _, s := range p.Stops {
		doc.Stops = append(doc.Stops, stopRefDoc{StopID: s.StopID, Seq: s.Seq})
	}
	var prev profileDoc
	err := r.coll.FindOneAndReplace(ctx, bson.D{{Key: "_id", Value: p.RouteID}}, doc,
		options.FindOneAndReplace().SetUpsert(true).SetReturnDocument(options.Before)).Decode(&prev)
	p.UpdatedAt = doc.UpdatedAt
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, mapErr(err)
	}
	old := prev.toDomain()
	return &old, nil
}

func (r *RouteProfileRepository) Delete(ctx context.Context, routeID int64) (domain.RouteProfile, error) {
	var doc profileDoc
	if err := r.coll.FindOneAndDelete(ctx, bson.D{{Key: "_id", Value: routeID}}).Decode(&doc); err != nil {
		return domain.RouteProfile{}, mapErr(err)
	}
	return doc.toDomain(), nil
}
