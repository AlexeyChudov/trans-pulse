package httpapi

import (
	"net/http"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

func (a *api) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	u, err := a.Auth.Register(r.Context(), req.Login, req.Password)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toUser(u))
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	token, u, err := a.Auth.Login(r.Context(), req.Login, req.Password)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, loginResponse{Token: token, User: toUser(u)})
}

func (a *api) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFrom(r.Context())
	if !ok {
		a.fail(w, r, domain.ErrUnauthorized)
		return
	}
	u, err := a.Auth.Me(r.Context(), claims.UserID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUser(u))
}
