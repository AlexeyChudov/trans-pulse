package httpapi

import (
	"net/http"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

func (a *api) listVehicles(w http.ResponseWriter, r *http.Request) {
	vehicles, err := a.Vehicles.List(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapList(vehicles, toVehicle))
}

func (a *api) createVehicle(w http.ResponseWriter, r *http.Request) {
	var req vehicleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	v, err := a.Vehicles.Create(r.Context(), req.FleetNumber, req.Model)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toVehicle(v))
}

func (a *api) setVehicleStatus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	var req vehicleStatusRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	v, err := a.Vehicles.SetStatus(r.Context(), id, domain.VehicleStatus(req.Status))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toVehicle(v))
}
