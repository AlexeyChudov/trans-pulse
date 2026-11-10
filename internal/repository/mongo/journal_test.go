package mongorepo_test

import (
	"errors"
	"testing"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	mongorepo "github.com/AlexeyChudov/trans-pulse/internal/repository/mongo"
)

func newJournal(t *testing.T) *mongorepo.JournalRepository {
	t.Helper()
	repo := mongorepo.NewJournalRepository(testDB(t))
	if err := repo.Create(ctx, &domain.TripJournal{TripID: 1}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return repo
}

func TestJournalCreateGetDelete(t *testing.T) {
	repo := newJournal(t)
	got, err := repo.Get(ctx, 1)
	if err != nil || got.TripID != 1 || len(got.Events) != 0 || got.CreatedAt.IsZero() {
		t.Fatalf("Get: %+v, %v", got, err)
	}
	if err := repo.Create(ctx, &domain.TripJournal{TripID: 1}); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("повторное создание: %v", err)
	}
	if _, err := repo.Get(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get несуществующего: %v", err)
	}
	if err := repo.Delete(ctx, 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Delete(ctx, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("повторное удаление: %v", err)
	}
}

func TestJournalAppendKeepsOrderAndAllowsEqualTimes(t *testing.T) {
	repo := newJournal(t)
	events := []domain.JournalEvent{
		{At: base, Type: domain.EventDeparted},
		{At: base.Add(5 * time.Minute), Type: domain.EventDelayDetected, DelaySeconds: 420},
		{At: base.Add(5 * time.Minute), Type: domain.EventArrivedAtStop, StopID: 12}, // то же время — допустимо
		{At: base.Add(9 * time.Minute), Type: domain.EventNote, Note: "пробка"},
	}
	for _, e := range events {
		if err := repo.AppendEvent(ctx, 1, e); err != nil {
			t.Fatalf("AppendEvent(%s): %v", e.Type, err)
		}
	}
	got, _ := repo.Get(ctx, 1)
	if len(got.Events) != 4 || got.Events[1].DelaySeconds != 420 || got.Events[2].StopID != 12 || got.Events[3].Note != "пробка" {
		t.Fatalf("события сохранены неверно: %+v", got.Events)
	}
	if got.UpdatedAt.Before(got.CreatedAt) {
		t.Fatalf("updated_at должен обновляться: %+v", got)
	}
}

func TestJournalEventEarlierThanLastIsConflict(t *testing.T) {
	repo := newJournal(t)
	repo.AppendEvent(ctx, 1, domain.JournalEvent{At: base.Add(10 * time.Minute), Type: domain.EventDeparted})
	err := repo.AppendEvent(ctx, 1, domain.JournalEvent{At: base, Type: domain.EventNote, Note: "раньше"})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ожидалось ErrConflict, получено %v", err)
	}
	if got, _ := repo.Get(ctx, 1); len(got.Events) != 1 {
		t.Fatalf("событие не должно добавиться: %+v", got.Events)
	}
}

func TestJournalAppendToMissingJournalIsNotFound(t *testing.T) {
	repo := mongorepo.NewJournalRepository(testDB(t))
	if err := repo.AppendEvent(ctx, 42, domain.JournalEvent{At: base, Type: domain.EventDeparted}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ожидалось ErrNotFound, получено %v", err)
	}
}

func TestJournalIsFullAtMaxEvents(t *testing.T) {
	repo := mongorepo.NewJournalRepository(testDB(t))
	full := &domain.TripJournal{TripID: 1}
	for i := 0; i < domain.MaxJournalEvents; i++ {
		full.Events = append(full.Events, domain.JournalEvent{At: base.Add(time.Duration(i) * time.Second), Type: domain.EventNote, Note: "x"})
	}
	if err := repo.Create(ctx, full); err != nil {
		t.Fatalf("Create с %d событиями: %v", domain.MaxJournalEvents, err)
	}
	err := repo.AppendEvent(ctx, 1, domain.JournalEvent{At: base.Add(time.Hour), Type: domain.EventNote, Note: "лишнее"})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ожидалось ErrConflict (журнал заполнен), получено %v", err)
	}
}

func TestJournalSchemaRejectsEventWithoutRequiredFields(t *testing.T) {
	repo := newJournal(t)
	err := repo.AppendEvent(ctx, 1, domain.JournalEvent{At: base, Type: domain.EventDelayDetected}) // нет delay_seconds
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("ожидалось ErrInvalidInput от схемы, получено %v", err)
	}
}
