package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

type api struct {
	Deps
	tokens TokenParser
	log    *slog.Logger
}

// NewRouter собирает chi-роутер со всеми маршрутами API.
func NewRouter(d Deps) http.Handler {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	a := &api{Deps: d, tokens: d.Tokens, log: d.Logger}

	r := chi.NewRouter()
	r.Use(middleware.RequestID, a.recoverer, a.logRequests)
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		// открытые
		r.Post("/auth/register", a.register)
		r.Post("/auth/login", a.login)
		r.Get("/routes", a.listRoutes)
		r.Get("/routes/{id}", a.getRoute)
		r.Get("/routes/{id}/stops", a.routeStops)
		r.Get("/stops", a.listStops)
		r.Get("/stop-points/near", a.nearStopPoints)
		r.Get("/stop-points/{id}", a.getStopPoint)
		r.Get("/route-profiles/{id}", a.getRouteProfile)

		// только с токеном
		r.Group(func(r chi.Router) {
			r.Use(a.authenticate)
			r.Get("/auth/me", a.me)

			// диспетчер и администратор
			r.Group(func(r chi.Router) {
				r.Use(a.requireRole(domain.RoleDispatcher, domain.RoleAdmin))
				r.Get("/vehicles", a.listVehicles)
				r.Get("/trips", a.listTrips)
				r.Get("/trips/{id}", a.getTrip)
				r.Get("/incidents", a.listIncidents)
				r.Post("/incidents/{id}/resolve", a.resolveIncident)

				// документные данные (MongoDB)
				r.Post("/telemetry", a.recordTelemetry)
				r.Get("/vehicles/{id}/telemetry", a.vehicleTelemetry)
				r.Post("/trips/{id}/journal", a.createJournal)
				r.Get("/trips/{id}/journal", a.getJournal)
				r.Post("/trips/{id}/journal/events", a.addJournalEvent)
				r.Post("/incidents/{id}/case", a.createCase)
				r.Get("/incidents/{id}/case", a.getCase)
				r.Patch("/incidents/{id}/case", a.updateCase)
				r.Post("/incidents/{id}/case/comments", a.addCaseComment)
				r.Post("/incidents/{id}/case/close", a.closeCase)
			})

			// только администратор
			r.Group(func(r chi.Router) {
				r.Use(a.requireRole(domain.RoleAdmin))
				r.Post("/routes", a.createRoute)
				r.Put("/routes/{id}", a.updateRoute)
				r.Delete("/routes/{id}", a.deleteRoute)
				r.Post("/vehicles", a.createVehicle)
				r.Patch("/vehicles/{id}", a.setVehicleStatus)
				r.Post("/trips", a.createTrip)
				r.Post("/trips/{id}/start", a.startTrip)
				r.Post("/trips/{id}/complete", a.completeTrip)
				r.Post("/trips/{id}/cancel", a.cancelTrip)
				r.Delete("/trips/{id}/journal", a.deleteJournal)
				r.Delete("/incidents/{id}/case", a.deleteCase)
				r.Post("/stop-points", a.createStopPoint)
				r.Put("/stop-points/{id}", a.setStopPointAttributes)
				r.Delete("/stop-points/{id}", a.deleteStopPoint)
				r.Put("/route-profiles/{id}/stops", a.setRouteStops)
				r.Delete("/route-profiles/{id}", a.deleteRouteProfile)
			})
		})
	})
	return r
}
