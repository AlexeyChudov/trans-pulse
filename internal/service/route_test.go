package service_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/service"
)

func TestRoutesListReturnsOnlyActive(t *testing.T) {
	repo := newFakeRoutes()
	repo.add("1", true)
	repo.add("2", false)
	list, err := service.NewRouteService(repo).List(ctx)
	if err != nil || len(list) != 1 || list[0].Number != "1" {
		t.Fatalf("получено %+v, %v", list, err)
	}
}

func TestRouteCreateValidatesInput(t *testing.T) {
	s := service.NewRouteService(newFakeRoutes())
	cases := map[string][2]string{
		"пустой номер":  {"  ", "Центр"},
		"пустое имя":    {"7", ""},
		"длинный номер": {strings.Repeat("9", 21), "Центр"},
		"длинное имя":   {"7", strings.Repeat("я", 101)},
	}
	for name, c := range cases {
		if _, err := s.Create(ctx, c[0], c[1]); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: ожидалось ErrInvalidInput, получено %v", name, err)
		}
	}
}

func TestRouteCreateDuplicateNumberConflicts(t *testing.T) {
	s := service.NewRouteService(newFakeRoutes())
	if _, err := s.Create(ctx, "7", "Центр — Вокзал"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, "7", "Другой"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ожидалось ErrConflict, получено %v", err)
	}
}

func TestRouteUpdateChangesFields(t *testing.T) {
	repo := newFakeRoutes()
	r := repo.add("1", true)
	got, err := service.NewRouteService(repo).Update(ctx, r.ID, "1A", "Новое имя", false)
	if err != nil || got.Number != "1A" || got.IsActive {
		t.Fatalf("получено %+v, %v", got, err)
	}
}

func TestRouteDeleteUsedByTripsConflicts(t *testing.T) {
	repo := newFakeRoutes()
	r := repo.add("1", true)
	repo.inUse[r.ID] = true
	if err := service.NewRouteService(repo).Delete(ctx, r.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ожидалось ErrConflict, получено %v", err)
	}
}

func TestRouteStopsOfUnknownRouteIsNotFound(t *testing.T) {
	if _, err := service.NewRouteService(newFakeRoutes()).Stops(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ожидалось ErrNotFound, получено %v", err)
	}
}
