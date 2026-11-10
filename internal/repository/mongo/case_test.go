package mongorepo_test

import (
	"errors"
	"testing"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	mongorepo "github.com/AlexeyChudov/trans-pulse/internal/repository/mongo"
)

func newCase(t *testing.T) *mongorepo.CaseRepository {
	t.Helper()
	repo := mongorepo.NewCaseRepository(testDB(t))
	c := &domain.IncidentCase{IncidentID: 1, TripID: 2, VehicleID: 3, RouteID: 4, Title: "Задержка рейса", Tags: []string{"delay", "traffic"}, CreatedBy: 7}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return repo
}

func TestCaseCreateGetRoundTrip(t *testing.T) {
	repo := newCase(t)
	got, err := repo.Get(ctx, 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.IncidentID != 1 || got.TripID != 2 || got.VehicleID != 3 || got.RouteID != 4 || got.Title != "Задержка рейса" ||
		len(got.Tags) != 2 || got.Status != domain.CaseOpen || got.CreatedBy != 7 || got.CreatedAt.IsZero() || got.ClosedAt != nil || len(got.Comments) != 0 {
		t.Fatalf("получено %+v", got)
	}
	if err := repo.Create(ctx, &domain.IncidentCase{IncidentID: 1, Title: "дубль", CreatedBy: 7}); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("повторное создание: %v", err)
	}
	if _, err := repo.Get(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get несуществующей: %v", err)
	}
}

func TestCaseAddCommentAppendsInOrder(t *testing.T) {
	repo := newCase(t)
	repo.AddComment(ctx, 1, domain.CaseComment{AuthorID: 7, Text: "первый", At: base})
	got, err := repo.AddComment(ctx, 1, domain.CaseComment{AuthorID: 8, Text: "второй", At: base.Add(time.Minute)})
	if err != nil || len(got.Comments) != 2 || got.Comments[0].Text != "первый" || got.Comments[1].AuthorID != 8 {
		t.Fatalf("получено %+v, %v", got, err)
	}
	if _, err := repo.AddComment(ctx, 99, domain.CaseComment{AuthorID: 7, Text: "x", At: base}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("комментарий к несуществующей карточке: %v", err)
	}
}

func TestCaseCloseThenCommentsAndEditsAreConflicts(t *testing.T) {
	repo := newCase(t)
	closed, err := repo.Close(ctx, 1, 7, base)
	if err != nil || closed.Status != domain.CaseClosed || closed.ClosedAt == nil || closed.ClosedBy == nil || *closed.ClosedBy != 7 {
		t.Fatalf("Close: %+v, %v", closed, err)
	}
	if _, err := repo.Close(ctx, 1, 7, base); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("повторное закрытие: %v", err)
	}
	if _, err := repo.AddComment(ctx, 1, domain.CaseComment{AuthorID: 7, Text: "поздно", At: base}); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("комментарий к закрытой карточке: %v", err)
	}
	title := "новое имя"
	if _, err := repo.Update(ctx, 1, &title, nil, base); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("правка закрытой карточки: %v", err)
	}
}

func TestCaseUpdateChangesOnlyGivenFields(t *testing.T) {
	repo := newCase(t)
	title := "Сбой GPS"
	got, err := repo.Update(ctx, 1, &title, nil, base.Add(time.Hour))
	if err != nil || got.Title != "Сбой GPS" || len(got.Tags) != 2 || !got.UpdatedAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("только заголовок: %+v, %v", got, err)
	}
	got, err = repo.Update(ctx, 1, nil, []string{"gps"}, base.Add(2*time.Hour))
	if err != nil || got.Title != "Сбой GPS" || len(got.Tags) != 1 || got.Tags[0] != "gps" {
		t.Fatalf("только теги: %+v, %v", got, err)
	}
	if _, err := repo.Update(ctx, 99, &title, nil, base); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("несуществующая: %v", err)
	}
}

func TestCaseCommentLimit(t *testing.T) {
	db := testDB(t)
	repo := mongorepo.NewCaseRepository(db)
	c := &domain.IncidentCase{IncidentID: 1, Title: "Много комментариев", CreatedBy: 7}
	for i := 0; i < domain.MaxCaseComments; i++ {
		c.Comments = append(c.Comments, domain.CaseComment{AuthorID: 7, Text: "x", At: base})
	}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := repo.AddComment(ctx, 1, domain.CaseComment{AuthorID: 7, Text: "лишний", At: base}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ожидалось ErrConflict (лимит комментариев), получено %v", err)
	}
}

func TestCaseDelete(t *testing.T) {
	repo := newCase(t)
	if err := repo.Delete(ctx, 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Delete(ctx, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("повторное удаление: %v", err)
	}
}
