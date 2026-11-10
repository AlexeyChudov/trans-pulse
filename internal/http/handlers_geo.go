package httpapi

import (
	"net/http"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

func (a *api) nearStopPoints(w http.ResponseWriter, r *http.Request) {
	lat, err := queryFloatRequired(r, "lat")
	if err != nil {
		a.fail(w, r, err)
		return
	}
	lon, err := queryFloatRequired(r, "lon")
	if err != nil {
		a.fail(w, r, err)
		return
	}
	radius, err := queryInt(r, "radius")
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if radius == 0 {
		radius = 500 // по умолчанию 500 м
	}
	limit, err := queryInt(r, "limit")
	if err != nil {
		a.fail(w, r, err)
		return
	}
	found, err := a.Geo.Near(r.Context(), lat, lon, radius, limit)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapList(found, func(n domain.NearStop) nearStopResponse {
		return nearStopResponse{stopPointResponse: toStopPoint(n.StopPoint), DistanceM: n.DistanceM}
	}))
}

func (a *api) getStopPoint(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	s, err := a.Geo.GetStopPoint(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toStopPoint(s))
}

func (a *api) createStopPoint(w http.ResponseWriter, r *http.Request) {
	var req stopPointCreateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	s, err := a.Geo.CreateStopPoint(r.Context(), req.StopID, normalizeAttributes(req.Attributes))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toStopPoint(s))
}

func (a *api) setStopPointAttributes(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	var req stopPointAttributesRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	s, err := a.Geo.SetAttributes(r.Context(), id, normalizeAttributes(req.Attributes))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toStopPoint(s))
}

func (a *api) deleteStopPoint(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if err := a.Geo.DeleteStopPoint(r.Context(), id); err != nil {
		a.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) getRouteProfile(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	p, err := a.Geo.GetRouteProfile(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRouteProfile(p))
}

func (a *api) setRouteStops(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	var req routeStopsRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	refs := mapList(req.Stops, func(s routeStopRefDTO) domain.RouteStopRef { return domain.RouteStopRef{StopID: s.StopID, Seq: s.Seq} })
	p, err := a.Geo.SetRouteStops(r.Context(), id, refs)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRouteProfile(p))
}

func (a *api) deleteRouteProfile(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if err := a.Geo.DeleteRouteProfile(r.Context(), id); err != nil {
		a.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
