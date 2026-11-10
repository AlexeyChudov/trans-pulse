package httpapi

import (
	"net/http"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

// currentUser возвращает идентификатор пользователя из токена.
func (a *api) currentUser(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims, ok := claimsFrom(r.Context())
	if !ok {
		a.fail(w, r, domain.ErrUnauthorized)
		return 0, false
	}
	return claims.UserID, true
}

func (a *api) createCase(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	user, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	var req caseCreateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	c, err := a.Cases.Create(r.Context(), id, user, req.Title, req.Tags)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCase(c))
}

func (a *api) getCase(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	c, err := a.Cases.Get(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toCase(c))
}

func (a *api) updateCase(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	var req caseUpdateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	c, err := a.Cases.Update(r.Context(), id, req.Title, req.Tags)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toCase(c))
}

func (a *api) addCaseComment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	user, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	var req commentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	c, err := a.Cases.AddComment(r.Context(), id, user, req.Text)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCase(c))
}

func (a *api) closeCase(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	user, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	c, err := a.Cases.Close(r.Context(), id, user)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toCase(c))
}

func (a *api) deleteCase(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if err := a.Cases.Delete(r.Context(), id); err != nil {
		a.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
