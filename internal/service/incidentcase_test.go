package service_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/service"
)

type caseEnv struct {
	cases     *fakeCases
	incidents *fakeIncidents
	trips     *fakeTrips
	svc       *service.CaseService
}

func newCaseEnv() caseEnv {
	e := caseEnv{cases: newFakeCases(), incidents: newFakeIncidents(), trips: newFakeTrips()}
	e.trips.Create(ctx, &domain.Trip{RouteID: 4, VehicleID: 3, Status: domain.TripInProgress}) // рейс 1
	e.incidents.add(1, domain.IncidentOpen)                                                    // инцидент 1 → рейс 1
	e.svc = service.NewCaseService(e.cases, e.incidents, e.trips, func() time.Time { return t0 })
	return e
}

func TestCaseCreateCopiesReferencesFromPostgres(t *testing.T) {
	e := newCaseEnv()
	c, err := e.svc.Create(ctx, 1, 7, "Задержка рейса", []string{"Delay", " traffic "})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.IncidentID != 1 || c.TripID != 1 || c.VehicleID != 3 || c.RouteID != 4 || c.CreatedBy != 7 || c.Status != domain.CaseOpen {
		t.Fatalf("ссылки или автор заполнены неверно: %+v", c)
	}
	if strings.Join(c.Tags, ",") != "delay,traffic" {
		t.Fatalf("теги должны нормализоваться (нижний регистр, без пробелов): %v", c.Tags)
	}
}

func TestCaseCreateRequiresExistingIncident(t *testing.T) {
	if _, err := newCaseEnv().svc.Create(ctx, 99, 7, "Нет такого инцидента", nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ожидалось ErrNotFound, получено %v", err)
	}
}

func TestCaseCreateDuplicateIsConflict(t *testing.T) {
	e := newCaseEnv()
	e.svc.Create(ctx, 1, 7, "Первая карточка", nil)
	if _, err := e.svc.Create(ctx, 1, 7, "Вторая карточка", nil); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ожидалось ErrConflict, получено %v", err)
	}
}

func TestCaseCreateValidatesTitleAndTags(t *testing.T) {
	e := newCaseEnv()
	cases := map[string]struct {
		title string
		tags  []string
	}{
		"короткий заголовок": {"ab", nil},
		"длинный заголовок":  {strings.Repeat("я", 201), nil},
		"дубликаты тегов":    {"Нормальный заголовок", []string{"a", "A"}},
		"больше 10 тегов":    {"Нормальный заголовок", []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}},
		"пустой тег":         {"Нормальный заголовок", []string{" "}},
		"длинный тег":        {"Нормальный заголовок", []string{strings.Repeat("т", 31)}},
	}
	for name, c := range cases {
		if _, err := e.svc.Create(ctx, 1, 7, c.title, c.tags); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: ожидалось ErrInvalidInput, получено %v", name, err)
		}
	}
}

func TestCaseAddCommentTrimsAndValidates(t *testing.T) {
	e := newCaseEnv()
	e.svc.Create(ctx, 1, 7, "Задержка рейса", nil)
	c, err := e.svc.AddComment(ctx, 1, 8, "  Водитель сообщил о пробке  ")
	if err != nil || len(c.Comments) != 1 || c.Comments[0].Text != "Водитель сообщил о пробке" || c.Comments[0].AuthorID != 8 || !c.Comments[0].At.Equal(t0) {
		t.Fatalf("AddComment: %+v, %v", c, err)
	}
	for name, text := range map[string]string{"пустой": "   ", "длинный": strings.Repeat("я", 2001)} {
		if _, err := e.svc.AddComment(ctx, 1, 8, text); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s комментарий: %v", name, err)
		}
	}
	if _, err := e.svc.AddComment(ctx, 99, 8, "нет карточки"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("несуществующая карточка: %v", err)
	}
}

func TestCaseCloseRequiresIncidentResolvedInPostgres(t *testing.T) {
	e := newCaseEnv()
	e.svc.Create(ctx, 1, 7, "Задержка рейса", nil)
	if _, err := e.svc.Close(ctx, 1, 7); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("инцидент ещё OPEN в PostgreSQL: ожидалось ErrConflict, получено %v", err)
	}
	e.incidents.Resolve(ctx, 1, 7, t0)
	c, err := e.svc.Close(ctx, 1, 7)
	if err != nil || c.Status != domain.CaseClosed || c.ClosedBy == nil || *c.ClosedBy != 7 || c.ClosedAt == nil {
		t.Fatalf("Close: %+v, %v", c, err)
	}
}

func TestCaseUpdateNeedsAtLeastOneField(t *testing.T) {
	e := newCaseEnv()
	e.svc.Create(ctx, 1, 7, "Задержка рейса", []string{"a"})
	if _, err := e.svc.Update(ctx, 1, nil, nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("пустое изменение: %v", err)
	}
	title := "Сбой GPS на маршруте"
	tags := []string{"GPS"}
	c, err := e.svc.Update(ctx, 1, &title, &tags)
	if err != nil || c.Title != title || len(c.Tags) != 1 || c.Tags[0] != "gps" {
		t.Fatalf("Update: %+v, %v", c, err)
	}
	short := "ab"
	if _, err := e.svc.Update(ctx, 1, &short, nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("короткий заголовок: %v", err)
	}
}

func TestCaseGetAndDelete(t *testing.T) {
	e := newCaseEnv()
	e.svc.Create(ctx, 1, 7, "Задержка рейса", nil)
	if _, err := e.svc.Get(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Delete(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Delete(ctx, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("повторное удаление: %v", err)
	}
}
