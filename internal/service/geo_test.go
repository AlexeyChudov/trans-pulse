package service_test

import (
	"errors"
	"testing"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/service"
)

type geoEnv struct {
	stopPoints *fakeStopPoints
	profiles   *fakeProfiles
	routes     *fakeRoutes
	svc        *service.GeoService
}

func newGeoEnv() geoEnv {
	e := geoEnv{stopPoints: newFakeStopPoints(), profiles: newFakeProfiles(), routes: newFakeRoutes()}
	e.routes.add("1", true) // маршрут 1
	e.routes.add("2", true) // маршрут 2
	for id, name := range map[int64]string{1: "Центральная", 2: "Вокзал", 3: "Университет"} {
		e.routes.stopsByID[id] = domain.Stop{ID: id, Name: name, Latitude: 50.45, Longitude: 30.52, IsActive: true}
	}
	e.svc = service.NewGeoService(e.stopPoints, e.profiles, e.routes)
	return e
}

func (e geoEnv) projectAll(t *testing.T) {
	t.Helper()
	for id := int64(1); id <= 3; id++ {
		if _, err := e.svc.CreateStopPoint(ctx, id, nil); err != nil {
			t.Fatalf("CreateStopPoint(%d): %v", id, err)
		}
	}
}

func TestStopPointIsProjectionOfPostgresStop(t *testing.T) {
	e := newGeoEnv()
	sp, err := e.svc.CreateStopPoint(ctx, 2, map[string]any{"shelter": true})
	if err != nil || sp.Name != "Вокзал" || sp.Latitude != 50.45 || sp.Longitude != 30.52 || sp.Attributes["shelter"] != true {
		t.Fatalf("имя и координаты должны копироваться из PostgreSQL: %+v, %v", sp, err)
	}
	if _, err := e.svc.CreateStopPoint(ctx, 99, nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("остановки нет в PostgreSQL: %v", err)
	}
	if _, err := e.svc.CreateStopPoint(ctx, 2, nil); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("повторное создание проекции: %v", err)
	}
}

func TestStopPointAttributesValidation(t *testing.T) {
	e := newGeoEnv()
	many := map[string]any{}
	for i := 0; i < 21; i++ {
		many[string(rune('a'+i))] = true
	}
	bad := map[string]map[string]any{
		"больше 20 атрибутов": many,
		"массив в значении":   {"x": []int{1}},
		"вложенный объект":    {"x": map[string]any{"y": 1}},
		"пустое имя атрибута": {"": true},
		"nil в значении":      {"x": nil},
	}
	for name, attrs := range bad {
		if _, err := e.svc.CreateStopPoint(ctx, 1, attrs); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: ожидалось ErrInvalidInput, получено %v", name, err)
		}
	}
	e.projectAll(t)
	if _, err := e.svc.SetAttributes(ctx, 1, map[string]any{"bench_count": 2, "surface": "asphalt", "lit": true, "width": 2.5}); err != nil {
		t.Errorf("допустимые атрибуты: %v", err)
	}
	if _, err := e.svc.SetAttributes(ctx, 99, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("несуществующая проекция: %v", err)
	}
}

func TestStopPointNearValidatesArguments(t *testing.T) {
	e := newGeoEnv()
	for name, args := range map[string][3]float64{
		"широта вне диапазона": {91, 30, 100}, "долгота вне диапазона": {50, 181, 100},
		"радиус меньше 1": {50, 30, 0}, "радиус больше 5000": {50, 30, 5001},
	} {
		if _, err := e.svc.Near(ctx, args[0], args[1], int(args[2]), 10); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: ожидалось ErrInvalidInput, получено %v", name, err)
		}
	}
	if _, err := e.svc.Near(ctx, 50.45, 30.52, 500, 0); err != nil {
		t.Errorf("корректный запрос с лимитом по умолчанию: %v", err)
	}
}

func TestSetRouteStopsMaintainsBackReferences(t *testing.T) {
	e := newGeoEnv()
	e.projectAll(t)

	p, err := e.svc.SetRouteStops(ctx, 1, []domain.RouteStopRef{{StopID: 2, Seq: 2}, {StopID: 1, Seq: 1}}) // порядок во входных данных произвольный
	if err != nil || len(p.Stops) != 2 || p.Stops[0].StopID != 1 || p.Stops[1].StopID != 2 {
		t.Fatalf("SetRouteStops: %+v, %v", p, err)
	}
	if got := e.stopPoints.routesOf(1); len(got) != 1 || got[0] != 1 {
		t.Fatalf("у остановки 1 должна появиться обратная ссылка на маршрут 1: %v", got)
	}

	// маршрут 2 тоже использует остановку 1 — связь M:N
	e.svc.SetRouteStops(ctx, 2, []domain.RouteStopRef{{StopID: 1, Seq: 1}, {StopID: 3, Seq: 2}})
	if got := e.stopPoints.routesOf(1); len(got) != 2 {
		t.Fatalf("остановка 1 входит в два маршрута: %v", got)
	}

	// у маршрута 1 убрали остановку 2: обратная ссылка должна исчезнуть, у остальных — остаться
	e.svc.SetRouteStops(ctx, 1, []domain.RouteStopRef{{StopID: 1, Seq: 1}})
	if got := e.stopPoints.routesOf(2); len(got) != 0 {
		t.Fatalf("остановку 2 убрали из маршрута 1: %v", got)
	}
	if got := e.stopPoints.routesOf(1); len(got) != 2 {
		t.Fatalf("остановка 1 по-прежнему в двух маршрутах: %v", got)
	}
}

func TestSetRouteStopsValidation(t *testing.T) {
	e := newGeoEnv()
	e.projectAll(t)
	many := make([]domain.RouteStopRef, domain.MaxRouteStops+1)
	for i := range many {
		many[i] = domain.RouteStopRef{StopID: int64(i + 1), Seq: i + 1}
	}
	bad := map[string][]domain.RouteStopRef{
		"пустой список":                  {},
		"нумерация не с 1":               {{StopID: 1, Seq: 2}},
		"пропуск в нумерации":            {{StopID: 1, Seq: 1}, {StopID: 2, Seq: 3}},
		"повтор номера":                  {{StopID: 1, Seq: 1}, {StopID: 2, Seq: 1}},
		"остановка дважды":               {{StopID: 1, Seq: 1}, {StopID: 1, Seq: 2}},
		"остановка без проекции в Mongo": {{StopID: 77, Seq: 1}},
		"больше 200 остановок":           many,
	}
	for name, stops := range bad {
		if _, err := e.svc.SetRouteStops(ctx, 1, stops); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: ожидалось ErrInvalidInput, получено %v", name, err)
		}
	}
	if len(e.profiles.byRoute) != 0 {
		t.Error("профиль не должен сохраняться при ошибке проверки")
	}
	if _, err := e.svc.SetRouteStops(ctx, 99, []domain.RouteStopRef{{StopID: 1, Seq: 1}}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("маршрута нет в PostgreSQL: %v", err)
	}
}

func TestDeleteRouteProfileRemovesBackReferencesAndUnblocksStopDeletion(t *testing.T) {
	e := newGeoEnv()
	e.projectAll(t)
	e.svc.SetRouteStops(ctx, 1, []domain.RouteStopRef{{StopID: 1, Seq: 1}, {StopID: 2, Seq: 2}})

	if err := e.svc.DeleteStopPoint(ctx, 1); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("остановка входит в маршрут: ожидалось ErrConflict, получено %v", err)
	}
	if err := e.svc.DeleteRouteProfile(ctx, 1); err != nil {
		t.Fatalf("DeleteRouteProfile: %v", err)
	}
	if got := e.stopPoints.routesOf(1); len(got) != 0 {
		t.Fatalf("обратные ссылки должны исчезнуть: %v", got)
	}
	if err := e.svc.DeleteStopPoint(ctx, 1); err != nil {
		t.Fatalf("теперь остановку можно удалить: %v", err)
	}
	if err := e.svc.DeleteRouteProfile(ctx, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("повторное удаление профиля: %v", err)
	}
}
