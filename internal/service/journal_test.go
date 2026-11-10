package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/service"
)

type journalEnv struct {
	journals *fakeJournals
	trips    *fakeTrips
	trip     domain.Trip
	svc      *service.JournalService
}

func newJournalEnv(t *testing.T) journalEnv {
	t.Helper()
	e := journalEnv{journals: newFakeJournals(), trips: newFakeTrips()}
	e.trips.Create(ctx, &domain.Trip{RouteID: 1, VehicleID: 1, Status: domain.TripInProgress})
	e.trip, _ = e.trips.Get(ctx, 1)
	e.svc = service.NewJournalService(e.journals, e.trips, func() time.Time { return t0 })
	return e
}

func TestJournalCreateRequiresExistingTripThatIsNotCancelled(t *testing.T) {
	e := newJournalEnv(t)
	if _, err := e.svc.Create(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("несуществующий рейс: %v", err)
	}
	e.trips.Create(ctx, &domain.Trip{RouteID: 1, VehicleID: 1, Status: domain.TripCancelled})
	if _, err := e.svc.Create(ctx, 2); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("отменённый рейс: %v", err)
	}
	j, err := e.svc.Create(ctx, e.trip.ID)
	if err != nil || j.TripID != e.trip.ID {
		t.Fatalf("Create: %+v, %v", j, err)
	}
	if _, err := e.svc.Create(ctx, e.trip.ID); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("повторное создание: %v", err)
	}
}

func TestJournalAddEventReturnsUpdatedJournal(t *testing.T) {
	e := newJournalEnv(t)
	e.svc.Create(ctx, e.trip.ID)
	j, err := e.svc.AddEvent(ctx, e.trip.ID, domain.JournalEvent{At: t0, Type: domain.EventDelayDetected, DelaySeconds: 360})
	if err != nil || len(j.Events) != 1 || j.Events[0].DelaySeconds != 360 {
		t.Fatalf("AddEvent: %+v, %v", j, err)
	}
}

func TestJournalAddEventValidation(t *testing.T) {
	cases := map[string]domain.JournalEvent{
		"неизвестный тип":                 {At: t0, Type: "TELEPORTED"},
		"нет времени":                     {Type: domain.EventDeparted},
		"время из будущего":               {At: t0.Add(time.Hour), Type: domain.EventDeparted},
		"DELAY_DETECTED без задержки":     {At: t0, Type: domain.EventDelayDetected},
		"задержка больше суток":           {At: t0, Type: domain.EventDelayDetected, DelaySeconds: 90000},
		"ARRIVED_AT_STOP без остановки":   {At: t0, Type: domain.EventArrivedAtStop},
		"задержка у события другого типа": {At: t0, Type: domain.EventDeparted, DelaySeconds: 10},
		"остановка у события DEPARTED":    {At: t0, Type: domain.EventDeparted, StopID: 5},
		"слишком длинная заметка":         {At: t0, Type: domain.EventNote, Note: string(make([]rune, 501))},
	}
	for name, ev := range cases {
		e := newJournalEnv(t)
		e.svc.Create(ctx, e.trip.ID)
		if _, err := e.svc.AddEvent(ctx, e.trip.ID, ev); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: ожидалось ErrInvalidInput, получено %v", name, err)
		}
	}
}

func TestJournalAddEventToJournalOfCancelledTripIsConflict(t *testing.T) {
	e := newJournalEnv(t)
	e.svc.Create(ctx, e.trip.ID)
	e.trips.byID[e.trip.ID].Status = domain.TripCancelled
	if _, err := e.svc.AddEvent(ctx, e.trip.ID, domain.JournalEvent{At: t0, Type: domain.EventDeparted}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ожидалось ErrConflict, получено %v", err)
	}
}

func TestJournalAddEventWithoutJournalIsNotFound(t *testing.T) {
	e := newJournalEnv(t)
	if _, err := e.svc.AddEvent(ctx, e.trip.ID, domain.JournalEvent{At: t0, Type: domain.EventDeparted}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ожидалось ErrNotFound, получено %v", err)
	}
}

func TestJournalGetAndDelete(t *testing.T) {
	e := newJournalEnv(t)
	e.svc.Create(ctx, e.trip.ID)
	if _, err := e.svc.Get(ctx, e.trip.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Delete(ctx, e.trip.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Get(ctx, e.trip.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("после удаления: %v", err)
	}
}
