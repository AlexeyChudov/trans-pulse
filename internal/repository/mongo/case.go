package mongorepo

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

// CaseRepository: карточка инцидента — один документ на инцидент (_id = номер инцидента в PostgreSQL).
// Рейс, автобус, маршрут и авторы хранятся ссылками (идентификаторами), комментарии встроены.
type CaseRepository struct{ coll *mongo.Collection }

func NewCaseRepository(db *mongo.Database) *CaseRepository {
	return &CaseRepository{coll: db.Collection(CollCases)}
}

type commentDoc struct {
	AuthorID int64     `bson:"author_id"`
	Text     string    `bson:"text"`
	At       time.Time `bson:"at"`
}

type caseDoc struct {
	ID            int64        `bson:"_id"`
	TripID        int64        `bson:"trip_id"`
	VehicleID     int64        `bson:"vehicle_id"`
	RouteID       int64        `bson:"route_id"`
	Title         string       `bson:"title"`
	Tags          []string     `bson:"tags"`
	Status        string       `bson:"status"`
	Comments      []commentDoc `bson:"comments"`
	CreatedBy     int64        `bson:"created_by"`
	CreatedAt     time.Time    `bson:"created_at"`
	UpdatedAt     time.Time    `bson:"updated_at"`
	ClosedAt      *time.Time   `bson:"closed_at,omitempty"`
	ClosedBy      *int64       `bson:"closed_by,omitempty"`
	SchemaVersion int32        `bson:"schema_version"`
}

func (d caseDoc) toDomain() domain.IncidentCase {
	c := domain.IncidentCase{IncidentID: d.ID, TripID: d.TripID, VehicleID: d.VehicleID, RouteID: d.RouteID, Title: d.Title,
		Tags: append([]string{}, d.Tags...), Status: domain.CaseStatus(d.Status), Comments: make([]domain.CaseComment, 0, len(d.Comments)),
		CreatedBy: d.CreatedBy, CreatedAt: d.CreatedAt.UTC(), UpdatedAt: d.UpdatedAt.UTC(), ClosedBy: d.ClosedBy}
	for _, cm := range d.Comments {
		c.Comments = append(c.Comments, domain.CaseComment{AuthorID: cm.AuthorID, Text: cm.Text, At: cm.At.UTC()})
	}
	if d.ClosedAt != nil {
		t := d.ClosedAt.UTC()
		c.ClosedAt = &t
	}
	return c
}

func (r *CaseRepository) Create(ctx context.Context, c *domain.IncidentCase) error {
	now := time.Now().UTC()
	doc := caseDoc{ID: c.IncidentID, TripID: c.TripID, VehicleID: c.VehicleID, RouteID: c.RouteID, Title: c.Title,
		Tags: append([]string{}, c.Tags...), Status: string(domain.CaseOpen), Comments: make([]commentDoc, 0, len(c.Comments)),
		CreatedBy: c.CreatedBy, CreatedAt: now, UpdatedAt: now, SchemaVersion: schemaVersion}
	for _, cm := range c.Comments {
		doc.Comments = append(doc.Comments, commentDoc{AuthorID: cm.AuthorID, Text: cm.Text, At: cm.At.UTC()})
	}
	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		return mapErr(err)
	}
	c.Status, c.CreatedAt, c.UpdatedAt = domain.CaseOpen, now, now
	return nil
}

func (r *CaseRepository) Get(ctx context.Context, incidentID int64) (domain.IncidentCase, error) {
	var doc caseDoc
	if err := r.coll.FindOne(ctx, bson.D{{Key: "_id", Value: incidentID}}).Decode(&doc); err != nil {
		return domain.IncidentCase{}, mapErr(err)
	}
	return doc.toDomain(), nil
}

// modifyOpen применяет update к ОТКРЫТОЙ карточке (дополнительные условия — в extra). Если карточка не изменена,
// выясняет причину: нет такой карточки (ErrNotFound) или нарушено условие (ErrConflict с пояснением).
func (r *CaseRepository) modifyOpen(ctx context.Context, incidentID int64, extra bson.D, update bson.D, why func(domain.IncidentCase) string) (domain.IncidentCase, error) {
	filter := append(bson.D{{Key: "_id", Value: incidentID}, {Key: "status", Value: string(domain.CaseOpen)}}, extra...)
	var doc caseDoc
	err := r.coll.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if err == nil {
		return doc.toDomain(), nil
	}
	if !isNoDocuments(err) {
		return domain.IncidentCase{}, mapErr(err)
	}
	current, getErr := r.Get(ctx, incidentID)
	if getErr != nil {
		return domain.IncidentCase{}, getErr
	}
	return domain.IncidentCase{}, fmt.Errorf("%w: %s", domain.ErrConflict, why(current))
}

func isNoDocuments(err error) bool { return err == mongo.ErrNoDocuments }

func reasonNotOpen(c domain.IncidentCase) string {
	if c.Status != domain.CaseOpen {
		return "карточка закрыта"
	}
	return "условие не выполнено"
}

func (r *CaseRepository) Update(ctx context.Context, incidentID int64, title *string, tags []string, at time.Time) (domain.IncidentCase, error) {
	set := bson.D{{Key: "updated_at", Value: at.UTC()}}
	if title != nil {
		set = append(set, bson.E{Key: "title", Value: *title})
	}
	if tags != nil {
		set = append(set, bson.E{Key: "tags", Value: tags})
	}
	return r.modifyOpen(ctx, incidentID, nil, bson.D{{Key: "$set", Value: set}}, reasonNotOpen)
}

func (r *CaseRepository) AddComment(ctx context.Context, incidentID int64, c domain.CaseComment) (domain.IncidentCase, error) {
	extra := bson.D{{Key: "comments." + strconv.Itoa(domain.MaxCaseComments-1), Value: bson.D{{Key: "$exists", Value: false}}}}
	update := bson.D{
		{Key: "$push", Value: bson.D{{Key: "comments", Value: commentDoc{AuthorID: c.AuthorID, Text: c.Text, At: c.At.UTC()}}}},
		{Key: "$set", Value: bson.D{{Key: "updated_at", Value: time.Now().UTC()}}},
	}
	return r.modifyOpen(ctx, incidentID, extra, update, func(cur domain.IncidentCase) string {
		if cur.Status != domain.CaseOpen {
			return "карточка закрыта, комментарии не принимаются"
		}
		return fmt.Sprintf("достигнут лимит комментариев (%d)", domain.MaxCaseComments)
	})
}

func (r *CaseRepository) Close(ctx context.Context, incidentID, userID int64, at time.Time) (domain.IncidentCase, error) {
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "status", Value: string(domain.CaseClosed)}, {Key: "closed_at", Value: at.UTC()},
		{Key: "closed_by", Value: userID}, {Key: "updated_at", Value: at.UTC()},
	}}}
	return r.modifyOpen(ctx, incidentID, nil, update, func(domain.IncidentCase) string { return "карточка уже закрыта" })
}

func (r *CaseRepository) Delete(ctx context.Context, incidentID int64) error {
	res, err := r.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: incidentID}})
	if err != nil {
		return mapErr(err)
	}
	if res.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}
