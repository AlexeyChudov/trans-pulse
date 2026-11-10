package httpapi

import (
	"context"
	"net/http"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

func (a *api) listTrips(w http.ResponseWriter, r *http.Request) {
	routeID, err := queryInt64(r, "route_id")
	if err != nil {
		a.fail(w, r, err)
		return
	}
	limit, err := queryInt(r, "limit")
	if err != nil {
		a.fail(w, r, err)
		return
	}
	offset, err := queryInt(r, "offset")
	if err != nil {
		a.fail(w, r, err)
		return
	}
	trips, err := a.Trips.List(r.Context(), domain.TripFilter{
		Status: domain.TripStatus(r.URL.Query().Get("status")), RouteID: routeID, Limit: limit, Offset: offset,
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapList(trips, toTrip))
}

func (a *api) getTrip(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	t, err := a.Trips.Get(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toTrip(t))
}

func (a *api) createTrip(w http.ResponseWriter, r *http.Request) {
	var req tripRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	t, err := a.Trips.Create(r.Context(), req.RouteID, req.VehicleID, req.PlannedStart, req.PlannedEnd)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toTrip(t))
}

func (a *api) startTrip(w http.ResponseWriter, r *http.Request) { a.tripAction(w, r, a.Trips.Start) }
func (a *api) completeTrip(w http.ResponseWriter, r *http.Request) {
	a.tripAction(w, r, a.Trips.Complete)
}
func (a *api) cancelTrip(w http.ResponseWriter, r *http.Request) { a.tripAction(w, r, a.Trips.Cancel) }

func (a *api) tripAction(w http.ResponseWriter, r *http.Request, action func(context.Context, int64) (domain.Trip, error)) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	t, err := action(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toTrip(t))
}
