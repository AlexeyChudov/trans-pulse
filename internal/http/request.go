package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

const maxBodyBytes = 1 << 20

func invalidInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalidInput, fmt.Sprintf(format, args...))
}

// decodeJSON читает тело запроса: ограничение размера, неизвестные поля и лишние данные запрещены.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return invalidInput("тело запроса: %v", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return invalidInput("тело запроса: лишние данные после JSON")
	}
	return nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, invalidInput("идентификатор в пути должен быть положительным числом")
	}
	return id, nil
}

func queryInt(r *http.Request, name string) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, invalidInput("параметр %s должен быть числом", name)
	}
	return v, nil
}

func queryInt64(r *http.Request, name string) (int64, error) {
	v, err := queryInt(r, name)
	return int64(v), err
}

func queryFloatRequired(r *http.Request, name string) (float64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, invalidInput("параметр %s обязателен", name)
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, invalidInput("параметр %s должен быть числом", name)
	}
	return v, nil
}

func queryTimeRequired(r *http.Request, name string) (time.Time, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return time.Time{}, invalidInput("параметр %s обязателен (формат RFC3339, например 2026-10-01T08:00:00Z)", name)
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, invalidInput("параметр %s: ожидается время в формате RFC3339", name)
	}
	return t, nil
}

// normalizeAttributes: JSON-числа приходят как float64; целые значения сохраняем как целые (2, а не 2.0).
func normalizeAttributes(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		if f, ok := v.(float64); ok && f == math.Trunc(f) && math.Abs(f) < 1<<53 {
			out[k] = int64(f)
		} else {
			out[k] = v
		}
	}
	return out
}
