package httpapi

import (
	"net/http"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

func (a *api) recordTelemetry(w http.ResponseWriter, r *http.Request) {
	var req telemetryRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	p := domain.TelemetryPoint{VehicleID: req.VehicleID, TripID: req.TripID, Timestamp: req.Timestamp,
		Latitude: req.Latitude, Longitude: req.Longitude, SpeedKmh: req.SpeedKmh}
	if err := a.Telemetry.Record(r.Context(), p); err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toTelemetry(p))
}

func (a *api) vehicleTelemetry(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	from, err := queryTimeRequired(r, "from")
	if err != nil {
		a.fail(w, r, err)
		return
	}
	to, err := queryTimeRequired(r, "to")
	if err != nil {
		a.fail(w, r, err)
		return
	}
	limit, err := queryInt(r, "limit")
	if err != nil {
		a.fail(w, r, err)
		return
	}
	points, err := a.Telemetry.History(r.Context(), id, from, to, limit)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapList(points, toTelemetry))
}
