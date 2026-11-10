package mongorepo_test

import (
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	mongorepo "github.com/AlexeyChudov/trans-pulse/internal/repository/mongo"
)

func point(vehicle int64, at time.Time) domain.TelemetryPoint {
	return domain.TelemetryPoint{VehicleID: vehicle, TripID: 9, Timestamp: at, Latitude: 50.45, Longitude: 30.52, SpeedKmh: 35}
}

func TestTelemetryPointsOfSameHourShareOneBucket(t *testing.T) {
	db := testDB(t)
	repo := mongorepo.NewTelemetryRepository(db)
	for i := 0; i < 3; i++ {
		if err := repo.Append(ctx, point(1, base.Add(time.Duration(i)*time.Minute))); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	n, _ := db.Collection("telemetry_buckets").CountDocuments(ctx, bson.M{})
	if n != 1 {
		t.Fatalf("ожидалась одна корзина на час, получено %d", n)
	}
	var doc struct {
		Count  int `bson:"count"`
		Points []bson.M
	}
	db.Collection("telemetry_buckets").FindOne(ctx, bson.M{}).Decode(&doc)
	if doc.Count != 3 || len(doc.Points) != 3 {
		t.Fatalf("count=%d, точек=%d", doc.Count, len(doc.Points))
	}

	repo.Append(ctx, point(1, base.Add(2*time.Hour))) // другой час — новая корзина
	repo.Append(ctx, point(2, base))                  // другой автобус — новая корзина
	if n, _ := db.Collection("telemetry_buckets").CountDocuments(ctx, bson.M{}); n != 3 {
		t.Fatalf("ожидалось 3 корзины, получено %d", n)
	}
}

func TestTelemetryHistoryIsSortedFilteredAndLimited(t *testing.T) {
	repo := mongorepo.NewTelemetryRepository(testDB(t))
	// точки в двух часовых корзинах, добавлены не по порядку
	for _, m := range []int{90, 5, 65, 20, 130} {
		repo.Append(ctx, point(1, base.Add(time.Duration(m)*time.Minute)))
	}
	repo.Append(ctx, point(2, base.Add(10*time.Minute))) // чужой автобус

	got, err := repo.History(ctx, 1, base, base.Add(2*time.Hour), 100)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(got) != 4 { // 5, 20, 65, 90 минут; 130 вне периода
		t.Fatalf("ожидалось 4 точки, получено %d: %+v", len(got), got)
	}
	for i := 1; i < len(got); i++ {
		if got[i].Timestamp.Before(got[i-1].Timestamp) {
			t.Fatalf("точки не отсортированы по времени: %+v", got)
		}
	}
	if got[0].VehicleID != 1 || got[0].TripID != 9 || got[0].SpeedKmh != 35 {
		t.Fatalf("поля точки потеряны: %+v", got[0])
	}
	limited, _ := repo.History(ctx, 1, base, base.Add(2*time.Hour), 2)
	if len(limited) != 2 || !limited[0].Timestamp.Equal(base.Add(5*time.Minute)) {
		t.Fatalf("limit: %+v", limited)
	}
	empty, err := repo.History(ctx, 1, base.Add(5*time.Hour), base.Add(6*time.Hour), 10)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("пустой период: %#v, %v", empty, err)
	}
}

func TestTelemetryFullBucketIsConflict(t *testing.T) {
	db := testDB(t)
	repo := mongorepo.NewTelemetryRepository(db)
	points := make(bson.A, domain.MaxBucketPoints)
	for i := range points {
		points[i] = bson.M{"ts": base.Add(time.Duration(i) * time.Second), "lat": 50.0, "lon": 30.0, "speed": 1.0}
	}
	err := insertRaw(db, "telemetry_buckets", bson.M{"vehicle_id": int64(1), "hour": base, "count": int32(domain.MaxBucketPoints), "points": points, "schema_version": int32(1)})
	if err != nil {
		t.Fatalf("подготовка заполненной корзины: %v", err)
	}
	if err := repo.Append(ctx, point(1, base.Add(time.Hour-time.Second))); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ожидалось ErrConflict, получено %v", err)
	}
}

func TestTelemetryInvalidPointIsRejectedBySchema(t *testing.T) {
	repo := mongorepo.NewTelemetryRepository(testDB(t))
	bad := point(1, base)
	bad.Latitude = 123
	if err := repo.Append(ctx, bad); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("схема валидации должна отклонить широту 123: %v", err)
	}
}
