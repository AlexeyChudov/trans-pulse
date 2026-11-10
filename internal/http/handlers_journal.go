package httpapi

import (
	"net/http"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

func (a *api) createJournal(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	j, err := a.Journals.Create(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toJournal(j))
}

func (a *api) getJournal(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	j, err := a.Journals.Get(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toJournal(j))
}

func (a *api) addJournalEvent(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	var req journalEventRequest
	if err := decodeJSON(w, r, &req); err != nil {
		a.fail(w, r, err)
		return
	}
	j, err := a.Journals.AddEvent(r.Context(), id, domain.JournalEvent{
		At: req.At, Type: domain.JournalEventType(req.Type), StopID: req.StopID, DelaySeconds: req.DelaySeconds, Note: req.Note,
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toJournal(j))
}

func (a *api) deleteJournal(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if err := a.Journals.Delete(r.Context(), id); err != nil {
		a.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
