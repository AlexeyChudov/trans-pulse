package mongorepo_test

import (
	"errors"
	"math"
	"testing"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	mongorepo "github.com/AlexeyChudov/trans-pulse/internal/repository/mongo"
)

func stop(id int64, name string, lat, lon float64) *domain.StopPoint {
	return &domain.StopPoint{StopID: id, Name: name, Latitude: lat, Longitude: lon}
}

func TestStopPointCreateGetAttributes(t *testing.T) {
	repo := mongorepo.NewStopPointRepository(testDB(t))
	if err := repo.Create(ctx, stop(1, "Вокзал", 50.4419, 30.5118)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Create(ctx, stop(1, "Дубль", 50.0, 30.0)); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("дубликат: %v", err)
	}
	got, err := repo.Get(ctx, 1)
	if err != nil || got.Name != "Вокзал" || got.Latitude != 50.4419 || got.Longitude != 30.5118 || len(got.RouteIDs) != 0 {
		t.Fatalf("Get: %+v, %v", got, err)
	}
	upd, err := repo.SetAttributes(ctx, 1, map[string]any{"shelter": true, "bench_count": int64(2), "surface": "asphalt"}, base)
	if err != nil || upd.Attributes["shelter"] != true || upd.Attributes["surface"] != "asphalt" {
		t.Fatalf("SetAttributes: %+v, %v", upd, err)
	}
	if _, err := repo.SetAttributes(ctx, 99, map[string]any{}, base); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("атрибуты несуществующей: %v", err)
	}
	if _, err := repo.Get(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get несуществующей: %v", err)
	}
}

func TestStopPointSchemaRejectsBadCoordinatesAndAttributes(t *testing.T) {
	repo := mongorepo.NewStopPointRepository(testDB(t))
	if err := repo.Create(ctx, stop(1, "Нигде", 95, 30)); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("широта 95: %v", err)
	}
	repo.Create(ctx, stop(2, "Парк", 50.45, 30.52))
	if _, err := repo.SetAttributes(ctx, 2, map[string]any{"list": []int{1}}, base); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("атрибут-массив: %v", err)
	}
}

func TestStopPointNearSortedByDistanceWithinRadius(t *testing.T) {
	repo := mongorepo.NewStopPointRepository(testDB(t))
	repo.Create(ctx, stop(1, "Далеко", 50.4600, 30.5200))       // ~1 км к северу
	repo.Create(ctx, stop(2, "Рядом", 50.4505, 30.5200))        // ~55 м
	repo.Create(ctx, stop(3, "Средне", 50.4550, 30.5200))       // ~550 м
	repo.Create(ctx, stop(4, "Очень далеко", 50.6000, 30.5200)) // ~16 км

	got, err := repo.Near(ctx, 50.4500, 30.5200, 1500, 10)
	if err != nil {
		t.Fatalf("Near: %v", err)
	}
	if len(got) != 3 || got[0].StopID != 2 || got[1].StopID != 3 || got[2].StopID != 1 {
		t.Fatalf("неверный порядок или состав: %+v", got)
	}
	if math.Abs(got[0].DistanceM-55.6) > 3 || math.Abs(got[1].DistanceM-556) > 10 {
		t.Fatalf("неверные расстояния: %.1f, %.1f", got[0].DistanceM, got[1].DistanceM)
	}
	limited, _ := repo.Near(ctx, 50.4500, 30.5200, 1500, 1)
	if len(limited) != 1 || limited[0].StopID != 2 {
		t.Fatalf("limit: %+v", limited)
	}
	none, err := repo.Near(ctx, 10, 10, 1000, 5)
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("пустой результат: %#v, %v", none, err)
	}
}

func TestStopPointRouteBackReferencesAreIdempotentAndProtectDeletion(t *testing.T) {
	repo := mongorepo.NewStopPointRepository(testDB(t))
	repo.Create(ctx, stop(1, "Вокзал", 50.44, 30.51))

	for i := 0; i < 2; i++ { // повторное добавление не создаёт дубликат
		if err := repo.AddRoute(ctx, 1, 7); err != nil {
			t.Fatalf("AddRoute: %v", err)
		}
	}
	repo.AddRoute(ctx, 1, 8)
	got, _ := repo.Get(ctx, 1)
	if len(got.RouteIDs) != 2 {
		t.Fatalf("route_ids: %+v", got.RouteIDs)
	}
	if err := repo.Delete(ctx, 1); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("удаление остановки, входящей в маршруты: %v", err)
	}
	repo.RemoveRoute(ctx, 1, 7)
	repo.RemoveRoute(ctx, 1, 7) // идемпотентно
	repo.RemoveRoute(ctx, 1, 8)
	if err := repo.Delete(ctx, 1); err != nil {
		t.Errorf("удаление свободной остановки: %v", err)
	}
	if err := repo.Delete(ctx, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("повторное удаление: %v", err)
	}
	if err := repo.AddRoute(ctx, 99, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("AddRoute к несуществующей: %v", err)
	}
}

func TestRouteProfileReplaceReturnsPrevious(t *testing.T) {
	repo := mongorepo.NewRouteProfileRepository(testDB(t))
	first := &domain.RouteProfile{RouteID: 1, Stops: []domain.RouteStopRef{{StopID: 10, Seq: 1}, {StopID: 11, Seq: 2}}}
	prev, err := repo.Replace(ctx, first)
	if err != nil || prev != nil {
		t.Fatalf("первое сохранение: prev=%+v err=%v", prev, err)
	}
	second := &domain.RouteProfile{RouteID: 1, Stops: []domain.RouteStopRef{{StopID: 11, Seq: 1}}}
	prev, err = repo.Replace(ctx, second)
	if err != nil || prev == nil || len(prev.Stops) != 2 || prev.Stops[0].StopID != 10 {
		t.Fatalf("второе сохранение: prev=%+v err=%v", prev, err)
	}
	got, err := repo.Get(ctx, 1)
	if err != nil || len(got.Stops) != 1 || got.Stops[0].StopID != 11 || got.UpdatedAt.IsZero() {
		t.Fatalf("Get: %+v, %v", got, err)
	}
	if _, err := repo.Get(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get несуществующего: %v", err)
	}
	deleted, err := repo.Delete(ctx, 1)
	if err != nil || len(deleted.Stops) != 1 {
		t.Fatalf("Delete: %+v, %v", deleted, err)
	}
	if _, err := repo.Delete(ctx, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("повторное удаление: %v", err)
	}
}

func TestRouteProfileSchemaRejectsZeroSequence(t *testing.T) {
	repo := mongorepo.NewRouteProfileRepository(testDB(t))
	_, err := repo.Replace(ctx, &domain.RouteProfile{RouteID: 1, Stops: []domain.RouteStopRef{{StopID: 1, Seq: 0}}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("ожидалось ErrInvalidInput, получено %v", err)
	}
}
