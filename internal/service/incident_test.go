package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/service"
)

func TestIncidentResolveRecordsResolverAndTime(t *testing.T) {
	repo := newFakeIncidents()
	inc := repo.add(1, domain.IncidentOpen)
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	s := service.NewIncidentService(repo, func() time.Time { return now })

	got, err := s.Resolve(ctx, inc.ID, 7)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Status != domain.IncidentResolved || got.ResolvedByUserID == nil || *got.ResolvedByUserID != 7 || !got.ResolvedAt.Equal(now) {
		t.Fatalf("получено %+v", got)
	}
}

func TestIncidentResolveTwiceConflicts(t *testing.T) {
	repo := newFakeIncidents()
	inc := repo.add(1, domain.IncidentOpen)
	s := service.NewIncidentService(repo, nil)
	s.Resolve(ctx, inc.ID, 7)
	if _, err := s.Resolve(ctx, inc.ID, 7); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ожидалось ErrConflict, получено %v", err)
	}
}

func TestIncidentResolveUnknownIsNotFound(t *testing.T) {
	if _, err := service.NewIncidentService(newFakeIncidents(), nil).Resolve(ctx, 99, 7); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ожидалось ErrNotFound, получено %v", err)
	}
}

func TestIncidentListRejectsUnknownStatusAndFilters(t *testing.T) {
	repo := newFakeIncidents()
	repo.add(1, domain.IncidentOpen)
	repo.add(1, domain.IncidentResolved)
	s := service.NewIncidentService(repo, nil)

	if _, err := s.List(ctx, domain.IncidentFilter{Status: "WEIRD"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("ожидалось ErrInvalidInput, получено %v", err)
	}
	open, err := s.List(ctx, domain.IncidentFilter{Status: domain.IncidentOpen})
	if err != nil || len(open) != 1 {
		t.Errorf("получено %+v, %v", open, err)
	}
}
