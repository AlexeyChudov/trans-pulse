package mongorepo

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

// JournalRepository: журнал рейса — один документ на рейс (_id = номер рейса в PostgreSQL), события встроены.
type JournalRepository struct{ coll *mongo.Collection }

func NewJournalRepository(db *mongo.Database) *JournalRepository {
	return &JournalRepository{coll: db.Collection(CollJournals)}
}

type eventDoc struct {
	At           time.Time `bson:"at"`
	Type         string    `bson:"type"`
	StopID       int64     `bson:"stop_id,omitempty"`
	DelaySeconds int       `bson:"delay_seconds,omitempty"`
	Note         string    `bson:"note,omitempty"`
}

type journalDoc struct {
	ID            int64      `bson:"_id"`
	Events        []eventDoc `bson:"events"`
	LastEventAt   *time.Time `bson:"last_event_at"`
	CreatedAt     time.Time  `bson:"created_at"`
	UpdatedAt     time.Time  `bson:"updated_at"`
	SchemaVersion int32      `bson:"schema_version"`
}

func toEventDoc(e domain.JournalEvent) eventDoc {
	return eventDoc{At: e.At.UTC(), Type: string(e.Type), StopID: e.StopID, DelaySeconds: e.DelaySeconds, Note: e.Note}
}

func (r *JournalRepository) Create(ctx context.Context, j *domain.TripJournal) error {
	now := time.Now().UTC()
	doc := journalDoc{ID: j.TripID, Events: make([]eventDoc, 0, len(j.Events)), CreatedAt: now, UpdatedAt: now, SchemaVersion: schemaVersion}
	for _, e := range j.Events {
		doc.Events = append(doc.Events, toEventDoc(e))
		at := e.At.UTC()
		doc.LastEventAt = &at
	}
	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		return mapErr(err)
	}
	j.CreatedAt, j.UpdatedAt = now, now
	return nil
}

func (r *JournalRepository) Get(ctx context.Context, tripID int64) (domain.TripJournal, error) {
	var doc journalDoc
	if err := r.coll.FindOne(ctx, bson.D{{Key: "_id", Value: tripID}}).Decode(&doc); err != nil {
		return domain.TripJournal{}, mapErr(err)
	}
	j := domain.TripJournal{TripID: doc.ID, Events: make([]domain.JournalEvent, 0, len(doc.Events)), CreatedAt: doc.CreatedAt.UTC(), UpdatedAt: doc.UpdatedAt.UTC()}
	for _, e := range doc.Events {
		j.Events = append(j.Events, domain.JournalEvent{At: e.At.UTC(), Type: domain.JournalEventType(e.Type), StopID: e.StopID, DelaySeconds: e.DelaySeconds, Note: e.Note})
	}
	return j, nil
}

// AppendEvent — один атомарный updateOne. Фильтр кодирует бизнес-правила: журнал существует, в массиве
// есть место и событие не раньше последнего ($not $gt совпадает и с пустым last_event_at).
func (r *JournalRepository) AppendEvent(ctx context.Context, tripID int64, e domain.JournalEvent) error {
	at := e.At.UTC()
	filter := bson.D{
		{Key: "_id", Value: tripID},
		{Key: "events." + strconv.Itoa(domain.MaxJournalEvents-1), Value: bson.D{{Key: "$exists", Value: false}}},
		{Key: "last_event_at", Value: bson.D{{Key: "$not", Value: bson.D{{Key: "$gt", Value: at}}}}},
	}
	update := bson.D{
		{Key: "$push", Value: bson.D{{Key: "events", Value: toEventDoc(e)}}},
		{Key: "$set", Value: bson.D{{Key: "last_event_at", Value: at}, {Key: "updated_at", Value: time.Now().UTC()}}},
	}
	res, err := r.coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return mapErr(err)
	}
	if res.MatchedCount > 0 {
		return nil
	}
	// Ничего не совпало: выясняем причину, чтобы вернуть точную ошибку.
	current, err := r.Get(ctx, tripID)
	if err != nil {
		return err
	}
	if len(current.Events) >= domain.MaxJournalEvents {
		return fmt.Errorf("%w: журнал заполнен (максимум %d событий)", domain.ErrConflict, domain.MaxJournalEvents)
	}
	return fmt.Errorf("%w: событие раньше последнего события журнала", domain.ErrConflict)
}

func (r *JournalRepository) Delete(ctx context.Context, tripID int64) error {
	res, err := r.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: tripID}})
	if err != nil {
		return mapErr(err)
	}
	if res.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}
