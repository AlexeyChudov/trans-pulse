package service

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

const maxDelaySeconds = 86400

type JournalService struct {
	journals domain.JournalRepository
	trips    domain.TripRepository
	now      func() time.Time
}

func NewJournalService(journals domain.JournalRepository, trips domain.TripRepository, now func() time.Time) *JournalService {
	return &JournalService{journals: journals, trips: trips, now: clockOrNow(now)}
}

// requireTrip: рейс должен существовать в PostgreSQL (источник истины) и не быть отменённым.
func (s *JournalService) requireTrip(ctx context.Context, tripID int64) error {
	t, err := s.trips.Get(ctx, tripID)
	if err != nil {
		return err
	}
	if t.Status == domain.TripCancelled {
		return conflict("рейс отменён, журнал не ведётся")
	}
	return nil
}

func (s *JournalService) Create(ctx context.Context, tripID int64) (domain.TripJournal, error) {
	if err := s.requireTrip(ctx, tripID); err != nil {
		return domain.TripJournal{}, err
	}
	j := domain.TripJournal{TripID: tripID}
	if err := s.journals.Create(ctx, &j); err != nil {
		return domain.TripJournal{}, err
	}
	return j, nil
}

func (s *JournalService) Get(ctx context.Context, tripID int64) (domain.TripJournal, error) {
	return s.journals.Get(ctx, tripID)
}

// AddEvent добавляет событие в конец журнала. Поля события зависят от типа: DELAY_DETECTED требует задержку,
// ARRIVED_AT_STOP — остановку, остальные типы не должны их содержать. Порядок по времени и лимит размера проверяет репозиторий.
func (s *JournalService) AddEvent(ctx context.Context, tripID int64, e domain.JournalEvent) (domain.TripJournal, error) {
	if err := s.validateEvent(e); err != nil {
		return domain.TripJournal{}, err
	}
	if err := s.requireTrip(ctx, tripID); err != nil {
		return domain.TripJournal{}, err
	}
	if err := s.journals.AppendEvent(ctx, tripID, e); err != nil {
		return domain.TripJournal{}, err
	}
	return s.journals.Get(ctx, tripID)
}

func (s *JournalService) Delete(ctx context.Context, tripID int64) error {
	return s.journals.Delete(ctx, tripID)
}

func (s *JournalService) validateEvent(e domain.JournalEvent) error {
	if !e.Type.IsValid() {
		return invalid("неизвестный тип события")
	}
	if e.At.IsZero() {
		return invalid("время события обязательно")
	}
	if e.At.After(s.now().Add(futureTolerance)) {
		return invalid("время события не может быть в будущем")
	}
	if utf8.RuneCountInString(e.Note) > 500 {
		return invalid("заметка: не более 500 символов")
	}
	switch e.Type {
	case domain.EventDelayDetected:
		if e.DelaySeconds < 1 || e.DelaySeconds > maxDelaySeconds {
			return invalid("для DELAY_DETECTED нужна задержка от 1 до %d секунд", maxDelaySeconds)
		}
		if e.StopID != 0 {
			return invalid("остановка допустима только для ARRIVED_AT_STOP")
		}
	case domain.EventArrivedAtStop:
		if e.StopID <= 0 {
			return invalid("для ARRIVED_AT_STOP нужна остановка")
		}
		if e.DelaySeconds != 0 {
			return invalid("задержка допустима только для DELAY_DETECTED")
		}
	default:
		if e.StopID != 0 || e.DelaySeconds != 0 {
			return invalid("для события %s остановка и задержка не задаются", e.Type)
		}
	}
	return nil
}
