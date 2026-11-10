package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// classify переводит ошибку домена в HTTP-статус и код. Неизвестные ошибки считаются внутренними.
func classify(err error) (status int, code string) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return http.StatusBadRequest, "invalid_input"
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusUnauthorized, "unauthorized"
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden, "forbidden"
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict, "conflict"
	}
	return http.StatusInternalServerError, "internal"
}

// fail отвечает ошибкой. Подробности внутренних ошибок пишутся в лог, но клиенту не отдаются.
func (a *api) fail(w http.ResponseWriter, r *http.Request, err error) {
	status, code := classify(err)
	message := err.Error()
	if status == http.StatusInternalServerError {
		a.log.Error("внутренняя ошибка", "error", err, "method", r.Method, "path", r.URL.Path)
		message = "внутренняя ошибка сервера"
	}
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}
