package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

type CaseService struct {
	cases     domain.CaseRepository
	incidents domain.IncidentRepository
	trips     domain.TripRepository
	now       func() time.Time
}

func NewCaseService(cases domain.CaseRepository, incidents domain.IncidentRepository, trips domain.TripRepository, now func() time.Time) *CaseService {
	return &CaseService{cases: cases, incidents: incidents, trips: trips, now: clockOrNow(now)}
}

// Create заводит карточку расследования. Инцидент должен существовать в PostgreSQL; рейс, автобус и маршрут
// сохраняются ссылками (идентификаторами), а не копиями данных.
func (s *CaseService) Create(ctx context.Context, incidentID, userID int64, title string, tags []string) (domain.IncidentCase, error) {
	title = strings.TrimSpace(title)
	if err := validateTitle(title); err != nil {
		return domain.IncidentCase{}, err
	}
	normalized, err := normalizeTags(tags)
	if err != nil {
		return domain.IncidentCase{}, err
	}
	incident, err := s.incidents.Get(ctx, incidentID)
	if err != nil {
		return domain.IncidentCase{}, err
	}
	trip, err := s.trips.Get(ctx, incident.TripID)
	if err != nil {
		return domain.IncidentCase{}, err
	}
	c := domain.IncidentCase{IncidentID: incidentID, TripID: trip.ID, VehicleID: trip.VehicleID, RouteID: trip.RouteID,
		Title: title, Tags: normalized, CreatedBy: userID}
	if err := s.cases.Create(ctx, &c); err != nil {
		return domain.IncidentCase{}, err
	}
	return c, nil
}

func (s *CaseService) Get(ctx context.Context, incidentID int64) (domain.IncidentCase, error) {
	return s.cases.Get(ctx, incidentID)
}

// Update меняет заголовок и/или теги (хотя бы одно поле обязательно).
func (s *CaseService) Update(ctx context.Context, incidentID int64, title *string, tags *[]string) (domain.IncidentCase, error) {
	if title == nil && tags == nil {
		return domain.IncidentCase{}, invalid("нужно указать title или tags")
	}
	var newTitle *string
	if title != nil {
		t := strings.TrimSpace(*title)
		if err := validateTitle(t); err != nil {
			return domain.IncidentCase{}, err
		}
		newTitle = &t
	}
	var newTags []string
	if tags != nil {
		n, err := normalizeTags(*tags)
		if err != nil {
			return domain.IncidentCase{}, err
		}
		newTags = n
	}
	return s.cases.Update(ctx, incidentID, newTitle, newTags, s.now())
}

func (s *CaseService) AddComment(ctx context.Context, incidentID, authorID int64, text string) (domain.IncidentCase, error) {
	text = strings.TrimSpace(text)
	if n := utf8.RuneCountInString(text); n < 1 || n > 2000 {
		return domain.IncidentCase{}, invalid("комментарий: 1–2000 символов")
	}
	return s.cases.AddComment(ctx, incidentID, domain.CaseComment{AuthorID: authorID, Text: text, At: s.now()})
}

// Close закрывает карточку. Правило между хранилищами: пока инцидент не закрыт в PostgreSQL (источник истины
// статуса), закрыть карточку нельзя.
func (s *CaseService) Close(ctx context.Context, incidentID, userID int64) (domain.IncidentCase, error) {
	incident, err := s.incidents.Get(ctx, incidentID)
	if err != nil {
		return domain.IncidentCase{}, err
	}
	if incident.Status != domain.IncidentResolved {
		return domain.IncidentCase{}, conflict("инцидент ещё не закрыт в PostgreSQL")
	}
	return s.cases.Close(ctx, incidentID, userID, s.now())
}

func (s *CaseService) Delete(ctx context.Context, incidentID int64) error {
	return s.cases.Delete(ctx, incidentID)
}

func validateTitle(title string) error {
	if n := utf8.RuneCountInString(title); n < 3 || n > 200 {
		return invalid("заголовок: 3–200 символов")
	}
	return nil
}

// normalizeTags приводит теги к нижнему регистру без пробелов по краям; дубликаты, пустые и длинные теги недопустимы.
func normalizeTags(tags []string) ([]string, error) {
	if len(tags) > 10 {
		return nil, invalid("не более 10 тегов")
	}
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, raw := range tags {
		t := strings.ToLower(strings.TrimSpace(raw))
		if n := utf8.RuneCountInString(t); n < 1 || n > 30 {
			return nil, invalid("тег: 1–30 символов")
		}
		if seen[t] {
			return nil, invalid("тег %q повторяется", t)
		}
		seen[t] = true
		out = append(out, t)
	}
	return out, nil
}
