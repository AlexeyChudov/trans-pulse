package httpapi

import (
	"net/http"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

func (a *api) listIncidents(w http.ResponseWriter, r *http.Request) {
	tripID, err := queryInt64(r, "trip_id")
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
	incidents, err := a.Incidents.List(r.Context(), domain.IncidentFilter{
		Status: domain.IncidentStatus(r.URL.Query().Get("status")), TripID: tripID, Limit: limit, Offset: offset,
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapList(incidents, toIncident))
}

// resolveIncident закрывает инцидент от имени пользователя из токена.
func (a *api) resolveIncident(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	claims, ok := claimsFrom(r.Context())
	if !ok {
		a.fail(w, r, domain.ErrUnauthorized)
		return
	}
	i, err := a.Incidents.Resolve(r.Context(), id, claims.UserID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toIncident(i))
}
