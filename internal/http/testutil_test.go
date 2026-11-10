package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	httpapi "github.com/AlexeyChudov/trans-pulse/internal/http"
	"github.com/AlexeyChudov/trans-pulse/internal/jwt"
)

const testSecret = "0123456789abcdef0123456789abcdef"

// Заглушки сервисов: встраивают интерфейс (нереализованные методы паникуют, если тест их вызовет случайно).

type stubAuth struct {
	httpapi.AuthService
	register func(login, password string) (domain.User, error)
	login    func(login, password string) (string, domain.User, error)
	me       func(id int64) (domain.User, error)
}

func (s stubAuth) Register(_ context.Context, l, p string) (domain.User, error) {
	return s.register(l, p)
}
func (s stubAuth) Login(_ context.Context, l, p string) (string, domain.User, error) {
	return s.login(l, p)
}
func (s stubAuth) Me(_ context.Context, id int64) (domain.User, error) { return s.me(id) }

type stubRoutes struct {
	httpapi.RouteService
	list   func() ([]domain.Route, error)
	get    func(id int64) (domain.Route, error)
	create func(number, name string) (domain.Route, error)
}

func (s stubRoutes) List(context.Context) ([]domain.Route, error)          { return s.list() }
func (s stubRoutes) Get(_ context.Context, id int64) (domain.Route, error) { return s.get(id) }
func (s stubRoutes) Create(_ context.Context, n, name string) (domain.Route, error) {
	return s.create(n, name)
}

type stubTrips struct {
	httpapi.TripService
	list   func(f domain.TripFilter) ([]domain.Trip, error)
	create func(route, vehicle int64, start, end time.Time) (domain.Trip, error)
	start  func(id int64) (domain.Trip, error)
}

func (s stubTrips) List(_ context.Context, f domain.TripFilter) ([]domain.Trip, error) {
	return s.list(f)
}
func (s stubTrips) Create(_ context.Context, r, v int64, a, b time.Time) (domain.Trip, error) {
	return s.create(r, v, a, b)
}
func (s stubTrips) Start(_ context.Context, id int64) (domain.Trip, error) { return s.start(id) }

type stubIncidents struct {
	httpapi.IncidentService
	resolve func(id, userID int64) (domain.Incident, error)
}

func (s stubIncidents) Resolve(_ context.Context, id, uid int64) (domain.Incident, error) {
	return s.resolve(id, uid)
}

type apiOption func(*httpapi.Deps)

func newAPI(t *testing.T, opts ...apiOption) (nethttp.Handler, *jwt.Manager) {
	t.Helper()
	tokens, err := jwt.NewManager(testSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	deps := httpapi.Deps{Tokens: tokens, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	for _, o := range opts {
		o(&deps)
	}
	return httpapi.NewRouter(deps), tokens
}

type response struct {
	Status int
	Body   map[string]any
	Raw    string
}

func do(t *testing.T, h nethttp.Handler, method, path, token string, body any) response {
	t.Helper()
	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		reader = bytes.NewBufferString(b)
	default:
		raw, _ := json.Marshal(b)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	resp := response{Status: rec.Code, Raw: rec.Body.String()}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp.Body) // массивы в Body не попадут, их проверяют по Raw
	return resp
}

func token(t *testing.T, m *jwt.Manager, userID int64, role domain.Role) string {
	t.Helper()
	tok, err := m.Generate(userID, string(role))
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func errorCode(r response) string {
	e, _ := r.Body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

// doRawAuth отправляет произвольное значение заголовка Authorization (без префикса Bearer).
func doRawAuth(t *testing.T, h nethttp.Handler, method, path, headerValue string) response {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", headerValue)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	resp := response{Status: rec.Code, Raw: rec.Body.String()}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp.Body)
	return resp
}
