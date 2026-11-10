package httpapi_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	httpapi "github.com/AlexeyChudov/trans-pulse/internal/http"
)

func withRoutes(s stubRoutes) apiOption { return func(d *httpapi.Deps) { d.Routes = s } }
func withTrips(s stubTrips) apiOption   { return func(d *httpapi.Deps) { d.Trips = s } }
func withAuth(s stubAuth) apiOption     { return func(d *httpapi.Deps) { d.Auth = s } }
func withIncidents(s stubIncidents) apiOption {
	return func(d *httpapi.Deps) { d.Incidents = s }
}

func emptyTrips() stubTrips {
	return stubTrips{list: func(domain.TripFilter) ([]domain.Trip, error) { return []domain.Trip{}, nil }}
}

func TestHealthIsPublic(t *testing.T) {
	h, _ := newAPI(t)
	if r := do(t, h, "GET", "/health", "", nil); r.Status != 200 {
		t.Fatalf("статус %d", r.Status)
	}
}

func TestPublicRoutesListNeedsNoToken(t *testing.T) {
	h, _ := newAPI(t, withRoutes(stubRoutes{list: func() ([]domain.Route, error) {
		return []domain.Route{{ID: 1, Number: "7", Name: "Центр — Вокзал", IsActive: true}}, nil
	}}))
	r := do(t, h, "GET", "/api/v1/routes", "", nil)
	if r.Status != 200 || !strings.Contains(r.Raw, `"number":"7"`) || !strings.Contains(r.Raw, `"id":1`) {
		t.Fatalf("получено %d %s", r.Status, r.Raw)
	}
}

func TestEmptyListIsJSONArray(t *testing.T) {
	h, _ := newAPI(t, withRoutes(stubRoutes{list: func() ([]domain.Route, error) { return nil, nil }}))
	if r := do(t, h, "GET", "/api/v1/routes", "", nil); strings.TrimSpace(r.Raw) != "[]" {
		t.Fatalf("ожидалось [], получено %q", r.Raw)
	}
}

func TestProtectedEndpointsRequireToken(t *testing.T) {
	h, _ := newAPI(t, withTrips(emptyTrips()))
	for _, path := range []string{"/api/v1/auth/me", "/api/v1/trips", "/api/v1/vehicles", "/api/v1/incidents"} {
		r := do(t, h, "GET", path, "", nil)
		if r.Status != 401 || errorCode(r) != "unauthorized" {
			t.Errorf("%s: получено %d %s", path, r.Status, r.Raw)
		}
	}
}

func TestInvalidTokenIsUnauthorized(t *testing.T) {
	h, _ := newAPI(t)
	for name, tok := range map[string]string{"мусор": "garbage", "чужая подпись": "eyJhbGciOiJIUzI1NiJ9.e30.xxx"} {
		if r := do(t, h, "GET", "/api/v1/auth/me", tok, nil); r.Status != 401 {
			t.Errorf("%s: статус %d", name, r.Status)
		}
	}
}

func TestMalformedAuthorizationHeaderIsUnauthorized(t *testing.T) {
	h, tokens := newAPI(t)
	tok := token(t, tokens, 1, domain.RoleAdmin)
	// схема не Bearer: токен передан напрямую, без префикса
	r := doRawAuth(t, h, "GET", "/api/v1/auth/me", tok)
	if r.Status != 401 {
		t.Fatalf("заголовок без Bearer: статус %d", r.Status)
	}
}

func TestDispatcherCannotUseAdminEndpoints(t *testing.T) {
	h, tokens := newAPI(t)
	tok := token(t, tokens, 2, domain.RoleDispatcher)
	cases := [][2]string{
		{"POST", "/api/v1/routes"}, {"PUT", "/api/v1/routes/1"}, {"DELETE", "/api/v1/routes/1"},
		{"POST", "/api/v1/vehicles"}, {"PATCH", "/api/v1/vehicles/1"},
		{"POST", "/api/v1/trips"}, {"POST", "/api/v1/trips/1/start"}, {"POST", "/api/v1/trips/1/complete"}, {"POST", "/api/v1/trips/1/cancel"},
	}
	for _, c := range cases {
		r := do(t, h, c[0], c[1], tok, `{}`)
		if r.Status != 403 || errorCode(r) != "forbidden" {
			t.Errorf("%s %s: получено %d %s", c[0], c[1], r.Status, r.Raw)
		}
	}
}

func TestDispatcherAndAdminCanListTrips(t *testing.T) {
	var got domain.TripFilter
	h, tokens := newAPI(t, withTrips(stubTrips{list: func(f domain.TripFilter) ([]domain.Trip, error) {
		got = f
		return []domain.Trip{{ID: 5, RouteID: 1, VehicleID: 2, Status: domain.TripPlanned}}, nil
	}}))

	for _, role := range []domain.Role{domain.RoleDispatcher, domain.RoleAdmin} {
		r := do(t, h, "GET", "/api/v1/trips?status=PLANNED&route_id=3&limit=10&offset=20", token(t, tokens, 1, role), nil)
		if r.Status != 200 || !strings.Contains(r.Raw, `"status":"PLANNED"`) {
			t.Fatalf("%s: получено %d %s", role, r.Status, r.Raw)
		}
	}
	if got.Status != domain.TripPlanned || got.RouteID != 3 || got.Limit != 10 || got.Offset != 20 {
		t.Fatalf("фильтр передан неверно: %+v", got)
	}
}

func TestBadQueryParametersAreRejected(t *testing.T) {
	h, tokens := newAPI(t, withTrips(emptyTrips()))
	tok := token(t, tokens, 1, domain.RoleDispatcher)
	for _, q := range []string{"limit=abc", "offset=-1x", "route_id=zero"} {
		if r := do(t, h, "GET", "/api/v1/trips?"+q, tok, nil); r.Status != 400 || errorCode(r) != "invalid_input" {
			t.Errorf("%s: получено %d %s", q, r.Status, r.Raw)
		}
	}
}

func TestLoginReturnsTokenAndHidesPasswordHash(t *testing.T) {
	h, _ := newAPI(t, withAuth(stubAuth{login: func(l, p string) (string, domain.User, error) {
		return "tok123", domain.User{ID: 4, Login: l, PasswordHash: "SECRET-HASH", Role: domain.RoleAdmin, IsActive: true}, nil
	}}))
	r := do(t, h, "POST", "/api/v1/auth/login", "", map[string]string{"login": "boss", "password": "correct-horse"})
	if r.Status != 200 || r.Body["token"] != "tok123" {
		t.Fatalf("получено %d %s", r.Status, r.Raw)
	}
	if strings.Contains(r.Raw, "SECRET-HASH") || strings.Contains(strings.ToLower(r.Raw), "password") {
		t.Fatalf("в ответе не должно быть пароля или хэша: %s", r.Raw)
	}
}

func TestLoginWithWrongCredentialsIsUnauthorized(t *testing.T) {
	h, _ := newAPI(t, withAuth(stubAuth{login: func(string, string) (string, domain.User, error) {
		return "", domain.User{}, domain.ErrUnauthorized
	}}))
	if r := do(t, h, "POST", "/api/v1/auth/login", "", map[string]string{"login": "x", "password": "y"}); r.Status != 401 {
		t.Fatalf("статус %d", r.Status)
	}
}

func TestRegisterReturnsCreatedUser(t *testing.T) {
	h, _ := newAPI(t, withAuth(stubAuth{register: func(l, p string) (domain.User, error) {
		return domain.User{ID: 9, Login: l, Role: domain.RoleDispatcher, IsActive: true}, nil
	}}))
	r := do(t, h, "POST", "/api/v1/auth/register", "", map[string]string{"login": "ivan", "password": "correct-horse"})
	if r.Status != 201 || r.Body["login"] != "ivan" || r.Body["role"] != "DISPATCHER" {
		t.Fatalf("получено %d %s", r.Status, r.Raw)
	}
}

func TestRequestBodyValidation(t *testing.T) {
	h, _ := newAPI(t, withAuth(stubAuth{register: func(string, string) (domain.User, error) { return domain.User{}, nil }}))
	cases := map[string]string{
		"не JSON":           `{not json`,
		"неизвестное поле":  `{"login":"ivan","password":"correct-horse","role":"ADMIN"}`,
		"лишние данные":     `{"login":"ivan","password":"correct-horse"} {}`,
		"пустое тело":       ``,
		"неверный тип поля": `{"login":123,"password":"x"}`,
	}
	for name, body := range cases {
		if r := do(t, h, "POST", "/api/v1/auth/register", "", body); r.Status != 400 || errorCode(r) != "invalid_input" {
			t.Errorf("%s: получено %d %s", name, r.Status, r.Raw)
		}
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrNotFound, 404, "not_found"},
		{domain.ErrConflict, 409, "conflict"},
		{domain.ErrInvalidInput, 400, "invalid_input"},
		{domain.ErrForbidden, 403, "forbidden"},
		{domain.ErrUnauthorized, 401, "unauthorized"},
		{errors.New("connection refused to 10.0.0.5"), 500, "internal"},
	}
	for _, c := range cases {
		err := c.err
		h, _ := newAPI(t, withRoutes(stubRoutes{get: func(int64) (domain.Route, error) { return domain.Route{}, err }}))
		r := do(t, h, "GET", "/api/v1/routes/1", "", nil)
		if r.Status != c.status || errorCode(r) != c.code {
			t.Errorf("%v: получено %d %s, ожидалось %d %s", c.err, r.Status, r.Raw, c.status, c.code)
		}
		if c.status == 500 && strings.Contains(r.Raw, "10.0.0.5") {
			t.Errorf("внутренняя ошибка не должна утекать клиенту: %s", r.Raw)
		}
	}
}

func TestInvalidPathIDIsBadRequest(t *testing.T) {
	h, _ := newAPI(t, withRoutes(stubRoutes{get: func(int64) (domain.Route, error) { return domain.Route{}, nil }}))
	for _, id := range []string{"abc", "0", "-5"} {
		if r := do(t, h, "GET", "/api/v1/routes/"+id, "", nil); r.Status != 400 {
			t.Errorf("id=%s: статус %d", id, r.Status)
		}
	}
}

func TestResolveIncidentUsesUserFromToken(t *testing.T) {
	var gotID, gotUser int64
	h, tokens := newAPI(t, withIncidents(stubIncidents{resolve: func(id, uid int64) (domain.Incident, error) {
		gotID, gotUser = id, uid
		return domain.Incident{ID: id, TripID: 1, Type: "DELAY", Status: domain.IncidentResolved}, nil
	}}))
	r := do(t, h, "POST", "/api/v1/incidents/12/resolve", token(t, tokens, 77, domain.RoleDispatcher), nil)
	if r.Status != 200 || r.Body["status"] != "RESOLVED" {
		t.Fatalf("получено %d %s", r.Status, r.Raw)
	}
	if gotID != 12 || gotUser != 77 {
		t.Fatalf("сервису переданы id=%d user=%d, ожидалось 12 и 77 (из токена)", gotID, gotUser)
	}
}

func TestCreateTripParsesBody(t *testing.T) {
	var routeID, vehicleID int64
	var start time.Time
	h, tokens := newAPI(t, withTrips(stubTrips{create: func(r, v int64, a, b time.Time) (domain.Trip, error) {
		routeID, vehicleID, start = r, v, a
		return domain.Trip{ID: 1, RouteID: r, VehicleID: v, PlannedStart: a, PlannedEnd: b, Status: domain.TripPlanned}, nil
	}}))
	body := map[string]any{"route_id": 3, "vehicle_id": 4, "planned_start": "2026-10-01T08:00:00Z", "planned_end": "2026-10-01T09:00:00Z"}
	r := do(t, h, "POST", "/api/v1/trips", token(t, tokens, 1, domain.RoleAdmin), body)
	if r.Status != 201 || r.Body["status"] != "PLANNED" {
		t.Fatalf("получено %d %s", r.Status, r.Raw)
	}
	if routeID != 3 || vehicleID != 4 || !start.Equal(time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("сервису переданы неверные данные: %d %d %v", routeID, vehicleID, start)
	}
}

func TestStartTripReturnsUpdatedTrip(t *testing.T) {
	h, tokens := newAPI(t, withTrips(stubTrips{start: func(id int64) (domain.Trip, error) {
		return domain.Trip{ID: id, Status: domain.TripInProgress}, nil
	}}))
	r := do(t, h, "POST", "/api/v1/trips/8/start", token(t, tokens, 1, domain.RoleAdmin), nil)
	if r.Status != 200 || r.Body["status"] != "IN_PROGRESS" {
		t.Fatalf("получено %d %s", r.Status, r.Raw)
	}
}

func TestMeReturnsCurrentUser(t *testing.T) {
	h, tokens := newAPI(t, withAuth(stubAuth{me: func(id int64) (domain.User, error) {
		return domain.User{ID: id, Login: "ivan", Role: domain.RoleDispatcher, IsActive: true}, nil
	}}))
	r := do(t, h, "GET", "/api/v1/auth/me", token(t, tokens, 5, domain.RoleDispatcher), nil)
	if r.Status != 200 || r.Body["login"] != "ivan" || r.Body["id"] != float64(5) {
		t.Fatalf("получено %d %s", r.Status, r.Raw)
	}
}

func TestPanicInHandlerBecomes500JSON(t *testing.T) {
	h, _ := newAPI(t, withRoutes(stubRoutes{list: func() ([]domain.Route, error) { panic("boom") }}))
	r := do(t, h, "GET", "/api/v1/routes", "", nil)
	if r.Status != 500 || errorCode(r) != "internal" || strings.Contains(r.Raw, "boom") {
		t.Fatalf("получено %d %s", r.Status, r.Raw)
	}
}
