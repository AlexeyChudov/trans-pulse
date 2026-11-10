package service_test

import (
	"context"
	"sort"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

// ---- телеметрия ----

type fakeTelemetry struct{ points []domain.TelemetryPoint }

func (f *fakeTelemetry) Append(_ context.Context, p domain.TelemetryPoint) error {
	f.points = append(f.points, p)
	return nil
}

func (f *fakeTelemetry) History(_ context.Context, vehicleID int64, from, to time.Time, limit int) ([]domain.TelemetryPoint, error) {
	out := []domain.TelemetryPoint{}
	for _, p := range f.points {
		if p.VehicleID == vehicleID && !p.Timestamp.Before(from) && p.Timestamp.Before(to) {
			out = append(out, p)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ---- журналы ----

type fakeJournals struct {
	byTrip map[int64]*domain.TripJournal
}

func newFakeJournals() *fakeJournals { return &fakeJournals{byTrip: map[int64]*domain.TripJournal{}} }

func (f *fakeJournals) Create(_ context.Context, j *domain.TripJournal) error {
	if _, ok := f.byTrip[j.TripID]; ok {
		return domain.ErrConflict
	}
	cp := *j
	f.byTrip[j.TripID] = &cp
	return nil
}

func (f *fakeJournals) Get(_ context.Context, tripID int64) (domain.TripJournal, error) {
	if j, ok := f.byTrip[tripID]; ok {
		return *j, nil
	}
	return domain.TripJournal{}, domain.ErrNotFound
}

func (f *fakeJournals) AppendEvent(_ context.Context, tripID int64, e domain.JournalEvent) error {
	j, ok := f.byTrip[tripID]
	if !ok {
		return domain.ErrNotFound
	}
	j.Events = append(j.Events, e)
	return nil
}

func (f *fakeJournals) Delete(_ context.Context, tripID int64) error {
	if _, ok := f.byTrip[tripID]; !ok {
		return domain.ErrNotFound
	}
	delete(f.byTrip, tripID)
	return nil
}

// ---- карточки инцидентов ----

type fakeCases struct {
	byID map[int64]*domain.IncidentCase
}

func newFakeCases() *fakeCases { return &fakeCases{byID: map[int64]*domain.IncidentCase{}} }

func (f *fakeCases) Create(_ context.Context, c *domain.IncidentCase) error {
	if _, ok := f.byID[c.IncidentID]; ok {
		return domain.ErrConflict
	}
	c.Status = domain.CaseOpen
	cp := *c
	f.byID[c.IncidentID] = &cp
	return nil
}

func (f *fakeCases) Get(_ context.Context, id int64) (domain.IncidentCase, error) {
	if c, ok := f.byID[id]; ok {
		return *c, nil
	}
	return domain.IncidentCase{}, domain.ErrNotFound
}

func (f *fakeCases) Update(_ context.Context, id int64, title *string, tags []string, at time.Time) (domain.IncidentCase, error) {
	c, ok := f.byID[id]
	if !ok {
		return domain.IncidentCase{}, domain.ErrNotFound
	}
	if title != nil {
		c.Title = *title
	}
	if tags != nil {
		c.Tags = tags
	}
	c.UpdatedAt = at
	return *c, nil
}

func (f *fakeCases) AddComment(_ context.Context, id int64, cm domain.CaseComment) (domain.IncidentCase, error) {
	c, ok := f.byID[id]
	if !ok {
		return domain.IncidentCase{}, domain.ErrNotFound
	}
	c.Comments = append(c.Comments, cm)
	return *c, nil
}

func (f *fakeCases) Close(_ context.Context, id, userID int64, at time.Time) (domain.IncidentCase, error) {
	c, ok := f.byID[id]
	if !ok {
		return domain.IncidentCase{}, domain.ErrNotFound
	}
	c.Status, c.ClosedAt, c.ClosedBy = domain.CaseClosed, &at, &userID
	return *c, nil
}

func (f *fakeCases) Delete(_ context.Context, id int64) error {
	if _, ok := f.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

// ---- остановки и профили маршрутов ----

type fakeStopPoints struct{ byID map[int64]*domain.StopPoint }

func newFakeStopPoints() *fakeStopPoints { return &fakeStopPoints{byID: map[int64]*domain.StopPoint{}} }

func (f *fakeStopPoints) Create(_ context.Context, s *domain.StopPoint) error {
	if _, ok := f.byID[s.StopID]; ok {
		return domain.ErrConflict
	}
	cp := *s
	cp.RouteIDs = []int64{}
	f.byID[s.StopID] = &cp
	return nil
}

func (f *fakeStopPoints) Get(_ context.Context, id int64) (domain.StopPoint, error) {
	if s, ok := f.byID[id]; ok {
		return *s, nil
	}
	return domain.StopPoint{}, domain.ErrNotFound
}

func (f *fakeStopPoints) SetAttributes(_ context.Context, id int64, attrs map[string]any, at time.Time) (domain.StopPoint, error) {
	s, ok := f.byID[id]
	if !ok {
		return domain.StopPoint{}, domain.ErrNotFound
	}
	s.Attributes, s.UpdatedAt = attrs, at
	return *s, nil
}

func (f *fakeStopPoints) Delete(_ context.Context, id int64) error {
	s, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	if len(s.RouteIDs) > 0 {
		return domain.ErrConflict
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeStopPoints) Near(_ context.Context, lat, lon float64, radiusM, limit int) ([]domain.NearStop, error) {
	return []domain.NearStop{}, nil
}

func (f *fakeStopPoints) AddRoute(_ context.Context, stopID, routeID int64) error {
	s, ok := f.byID[stopID]
	if !ok {
		return domain.ErrNotFound
	}
	for _, r := range s.RouteIDs {
		if r == routeID {
			return nil
		}
	}
	s.RouteIDs = append(s.RouteIDs, routeID)
	return nil
}

func (f *fakeStopPoints) RemoveRoute(_ context.Context, stopID, routeID int64) error {
	s, ok := f.byID[stopID]
	if !ok {
		return domain.ErrNotFound
	}
	kept := s.RouteIDs[:0]
	for _, r := range s.RouteIDs {
		if r != routeID {
			kept = append(kept, r)
		}
	}
	s.RouteIDs = kept
	return nil
}

func (f *fakeStopPoints) routesOf(stopID int64) []int64 {
	ids := append([]int64{}, f.byID[stopID].RouteIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

type fakeProfiles struct{ byRoute map[int64]domain.RouteProfile }

func newFakeProfiles() *fakeProfiles { return &fakeProfiles{byRoute: map[int64]domain.RouteProfile{}} }

func (f *fakeProfiles) Get(_ context.Context, id int64) (domain.RouteProfile, error) {
	if p, ok := f.byRoute[id]; ok {
		return p, nil
	}
	return domain.RouteProfile{}, domain.ErrNotFound
}

func (f *fakeProfiles) Replace(_ context.Context, p *domain.RouteProfile) (*domain.RouteProfile, error) {
	prev, had := f.byRoute[p.RouteID]
	f.byRoute[p.RouteID] = *p
	if !had {
		return nil, nil
	}
	return &prev, nil
}

func (f *fakeProfiles) Delete(_ context.Context, id int64) (domain.RouteProfile, error) {
	p, ok := f.byRoute[id]
	if !ok {
		return domain.RouteProfile{}, domain.ErrNotFound
	}
	delete(f.byRoute, id)
	return p, nil
}
