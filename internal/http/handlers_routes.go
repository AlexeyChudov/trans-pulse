package httpapi

import "net/http"

func (a *api) listRoutes(w http.ResponseWriter, r *http.Request) {
	routes, err := a.Routes.List(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapList(routes, toRoute))
}

func (a *api) getRoute(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	route, err := a.Routes.Get(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRoute(route))
}

func (a *api) routeStops(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	stops, err := a.Routes.Stops(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapList(stops, toRouteStop))
}

func (a *api) listStops(w http.ResponseWriter, r *http.Request) {
	stops, err := a.Routes.AllStops(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapList(stops, toStop))
}

func (a *api) createRoute(w http.ResponseWriter, r *http.Request) {
	var req routeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	route, err := a.Routes.Create(r.Context(), req.Number, req.Name)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toRoute(route))
}

func (a *api) updateRoute(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	var req routeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	if req.IsActive == nil {
		a.fail(w, r, invalidInput("поле is_active обязательно"))
		return
	}
	route, err := a.Routes.Update(r.Context(), id, req.Number, req.Name, *req.IsActive)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRoute(route))
}

func (a *api) deleteRoute(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if err := a.Routes.Delete(r.Context(), id); err != nil {
		a.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
