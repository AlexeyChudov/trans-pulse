package service_test

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

var ctx = context.Background()

// ---- токены ----

type fakeTokens struct{}

func (fakeTokens) Generate(userID int64, role string) (string, error) {
	return fmt.Sprintf("token-%d-%s", userID, role), nil
}

// ---- пользователи ----

type fakeUsers struct {
	byID map[int64]domain.User
	next int64
}

func newFakeUsers() *fakeUsers { return &fakeUsers{byID: map[int64]domain.User{}} }

func (f *fakeUsers) Create(_ context.Context, u *domain.User) error {
	for _, x := range f.byID {
		if x.Login == u.Login {
			return domain.ErrConflict
		}
	}
	f.next++
	u.ID = f.next
	f.byID[u.ID] = *u
	return nil
}

func (f *fakeUsers) GetByLogin(_ context.Context, login string) (domain.User, error) {
	for _, u := range f.byID {
		if u.Login == login {
			return u, nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

func (f *fakeUsers) GetByID(_ context.Context, id int64) (domain.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return domain.User{}, domain.ErrNotFound
}

// ---- маршруты ----

type fakeRoutes struct {
	byID  map[int64]domain.Route
	next  int64
	inUse map[int64]bool // на маршрут ссылаются рейсы: удалять нельзя
	stops map[int64][]domain.RouteStop
}

func newFakeRoutes() *fakeRoutes {
	return &fakeRoutes{byID: map[int64]domain.Route{}, inUse: map[int64]bool{}, stops: map[int64][]domain.RouteStop{}}
}

func (f *fakeRoutes) add(number string, active bool) domain.Route {
	f.next++
	r := domain.Route{ID: f.next, Number: number, Name: "Маршрут " + number, IsActive: active}
	f.byID[r.ID] = r
	return r
}

func (f *fakeRoutes) List(_ context.Context, onlyActive bool) ([]domain.Route, error) {
	var out []domain.Route
	for _, r := range f.byID {
		if !onlyActive || r.IsActive {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRoutes) Get(_ context.Context, id int64) (domain.Route, error) {
	if r, ok := f.byID[id]; ok {
		return r, nil
	}
	return domain.Route{}, domain.ErrNotFound
}

func (f *fakeRoutes) Create(_ context.Context, r *domain.Route) error {
	for _, x := range f.byID {
		if x.Number == r.Number {
			return domain.ErrConflict
		}
	}
	f.next++
	r.ID = f.next
	f.byID[r.ID] = *r
	return nil
}

func (f *fakeRoutes) Update(_ context.Context, r *domain.Route) error {
	if _, ok := f.byID[r.ID]; !ok {
		return domain.ErrNotFound
	}
	f.byID[r.ID] = *r
	return nil
}

func (f *fakeRoutes) Delete(_ context.Context, id int64) error {
	if _, ok := f.byID[id]; !ok {
		return domain.ErrNotFound
	}
	if f.inUse[id] {
		return domain.ErrConflict
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeRoutes) ListStops(_ context.Context, routeID int64) ([]domain.RouteStop, error) {
	return f.stops[routeID], nil
}

func (f *fakeRoutes) ListAllStops(context.Context) ([]domain.Stop, error) { return nil, nil }

// ---- автобусы ----

type fakeVehicles struct {
	byID map[int64]domain.Vehicle
	next int64
}

func newFakeVehicles() *fakeVehicles { return &fakeVehicles{byID: map[int64]domain.Vehicle{}} }

func (f *fakeVehicles) add(fleet string, status domain.VehicleStatus) domain.Vehicle {
	f.next++
	v := domain.Vehicle{ID: f.next, FleetNumber: fleet, Model: "Bus", Status: status}
	f.byID[v.ID] = v
	return v
}

func (f *fakeVehicles) List(context.Context) ([]domain.Vehicle, error) {
	var out []domain.Vehicle
	for _, v := range f.byID {
		out = append(out, v)
	}
	return out, nil
}

func (f *fakeVehicles) Get(_ context.Context, id int64) (domain.Vehicle, error) {
	if v, ok := f.byID[id]; ok {
		return v, nil
	}
	return domain.Vehicle{}, domain.ErrNotFound
}

func (f *fakeVehicles) Create(_ context.Context, v *domain.Vehicle) error {
	for _, x := range f.byID {
		if x.FleetNumber == v.FleetNumber {
			return domain.ErrConflict
		}
	}
	f.next++
	v.ID = f.next
	f.byID[v.ID] = *v
	return nil
}

func (f *fakeVehicles) UpdateStatus(_ context.Context, id int64, st domain.VehicleStatus) error {
	v, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	v.Status = st
	f.byID[id] = v
	return nil
}

// ---- рейсы ----

type fakeTrips struct {
	byID map[int64]*domain.Trip
	next int64
}

func newFakeTrips() *fakeTrips { return &fakeTrips{byID: map[int64]*domain.Trip{}} }

func (f *fakeTrips) List(_ context.Context, flt domain.TripFilter) ([]domain.Trip, error) {
	var out []domain.Trip
	for _, t := range f.byID {
		if (flt.Status == "" || t.Status == flt.Status) && (flt.RouteID == 0 || t.RouteID == flt.RouteID) {
			out = append(out, *t)
		}
	}
	return out, nil
}

func (f *fakeTrips) Get(_ context.Context, id int64) (domain.Trip, error) {
	if t, ok := f.byID[id]; ok {
		return *t, nil
	}
	return domain.Trip{}, domain.ErrNotFound
}

func (f *fakeTrips) Create(_ context.Context, t *domain.Trip) error {
	f.next++
	t.ID = f.next
	cp := *t
	f.byID[t.ID] = &cp
	return nil
}

func (f *fakeTrips) Transition(_ context.Context, id int64, from, to domain.TripStatus, at time.Time) (domain.Trip, error) {
	t, ok := f.byID[id]
	if !ok {
		return domain.Trip{}, domain.ErrNotFound
	}
	if t.Status != from {
		return domain.Trip{}, domain.ErrConflict
	}
	if to == domain.TripInProgress {
		for _, other := range f.byID {
			if other.ID != id && other.VehicleID == t.VehicleID && other.Status == domain.TripInProgress {
				return domain.Trip{}, domain.ErrConflict // у автобуса уже есть активный рейс
			}
		}
		t.ActualStart = &at
	}
	if to == domain.TripCompleted {
		t.ActualEnd = &at
	}
	t.Status = to
	return *t, nil
}

// ---- инциденты ----

type fakeIncidents struct {
	byID map[int64]*domain.Incident
}

func newFakeIncidents() *fakeIncidents { return &fakeIncidents{byID: map[int64]*domain.Incident{}} }

func (f *fakeIncidents) add(tripID int64, status domain.IncidentStatus) domain.Incident {
	i := &domain.Incident{ID: int64(len(f.byID) + 1), TripID: tripID, Type: "DELAY", Status: status}
	f.byID[i.ID] = i
	return *i
}

func (f *fakeIncidents) List(_ context.Context, flt domain.IncidentFilter) ([]domain.Incident, error) {
	var out []domain.Incident
	for _, i := range f.byID {
		if flt.Status == "" || i.Status == flt.Status {
			out = append(out, *i)
		}
	}
	return out, nil
}

func (f *fakeIncidents) Get(_ context.Context, id int64) (domain.Incident, error) {
	if i, ok := f.byID[id]; ok {
		return *i, nil
	}
	return domain.Incident{}, domain.ErrNotFound
}

func (f *fakeIncidents) Resolve(_ context.Context, id, userID int64, at time.Time) (domain.Incident, error) {
	i, ok := f.byID[id]
	if !ok {
		return domain.Incident{}, domain.ErrNotFound
	}
	if i.Status != domain.IncidentOpen {
		return domain.Incident{}, domain.ErrConflict
	}
	i.Status, i.ResolvedAt, i.ResolvedByUserID = domain.IncidentResolved, &at, &userID
	return *i, nil
}
