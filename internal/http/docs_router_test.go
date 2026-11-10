package httpapi_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	httpapi "github.com/AlexeyChudov/trans-pulse/internal/http"
)

// ---- заглушки новых сервисов ----

type stubTelemetry struct {
	httpapi.TelemetryService
	record  func(p domain.TelemetryPoint) error
	history func(vehicleID int64, from, to time.Time, limit int) ([]domain.TelemetryPoint, error)
}

func (s stubTelemetry) Record(_ context.Context, p domain.TelemetryPoint) error { return s.record(p) }
func (s stubTelemetry) History(_ context.Context, v int64, a, b time.Time, l int) ([]domain.TelemetryPoint, error) {
	return s.history(v, a, b, l)
}

type stubJournals struct {
	httpapi.JournalService
	create   func(tripID int64) (domain.TripJournal, error)
	addEvent func(tripID int64, e domain.JournalEvent) (domain.TripJournal, error)
	delete   func(tripID int64) error
}

func (s stubJournals) Create(_ context.Context, id int64) (domain.TripJournal, error) {
	return s.create(id)
}
func (s stubJournals) AddEvent(_ context.Context, id int64, e domain.JournalEvent) (domain.TripJournal, error) {
	return s.addEvent(id, e)
}
func (s stubJournals) Delete(_ context.Context, id int64) error { return s.delete(id) }

type stubCases struct {
	httpapi.CaseService
	create     func(incidentID, userID int64, title string, tags []string) (domain.IncidentCase, error)
	update     func(incidentID int64, title *string, tags *[]string) (domain.IncidentCase, error)
	addComment func(incidentID, authorID int64, text string) (domain.IncidentCase, error)
	close      func(incidentID, userID int64) (domain.IncidentCase, error)
}

func (s stubCases) Create(_ context.Context, i, u int64, t string, tags []string) (domain.IncidentCase, error) {
	return s.create(i, u, t, tags)
}
func (s stubCases) Update(_ context.Context, i int64, t *string, tags *[]string) (domain.IncidentCase, error) {
	return s.update(i, t, tags)
}
func (s stubCases) AddComment(_ context.Context, i, a int64, text string) (domain.IncidentCase, error) {
	return s.addComment(i, a, text)
}
func (s stubCases) Close(_ context.Context, i, u int64) (domain.IncidentCase, error) {
	return s.close(i, u)
}

type stubGeo struct {
	httpapi.GeoService
	near       func(lat, lon float64, radius, limit int) ([]domain.NearStop, error)
	createStop func(stopID int64, attrs map[string]any) (domain.StopPoint, error)
	setStops   func(routeID int64, refs []domain.RouteStopRef) (domain.RouteProfile, error)
	getStop    func(id int64) (domain.StopPoint, error)
}

func (s stubGeo) Near(_ context.Context, lat, lon float64, r, l int) ([]domain.NearStop, error) {
	return s.near(lat, lon, r, l)
}
func (s stubGeo) CreateStopPoint(_ context.Context, id int64, a map[string]any) (domain.StopPoint, error) {
	return s.createStop(id, a)
}
func (s stubGeo) SetRouteStops(_ context.Context, id int64, refs []domain.RouteStopRef) (domain.RouteProfile, error) {
	return s.setStops(id, refs)
}
func (s stubGeo) GetStopPoint(_ context.Context, id int64) (domain.StopPoint, error) {
	return s.getStop(id)
}

func withTelemetry(s stubTelemetry) apiOption { return func(d *httpapi.Deps) { d.Telemetry = s } }
func withJournals(s stubJournals) apiOption   { return func(d *httpapi.Deps) { d.Journals = s } }
func withCases(s stubCases) apiOption         { return func(d *httpapi.Deps) { d.Cases = s } }
func withGeo(s stubGeo) apiOption             { return func(d *httpapi.Deps) { d.Geo = s } }

var when = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

// ---- телеметрия ----

func TestPostTelemetryParsesBodyAndReturnsCreated(t *testing.T) {
	var got domain.TelemetryPoint
	h, tokens := newAPI(t, withTelemetry(stubTelemetry{record: func(p domain.TelemetryPoint) error { got = p; return nil }}))
	body := map[string]any{"vehicle_id": 7, "trip_id": 3, "ts": "2026-10-01T08:00:00Z", "latitude": 50.45, "longitude": 30.52, "speed_kmh": 41.5}
	r := do(t, h, "POST", "/api/v1/telemetry", token(t, tokens, 1, domain.RoleDispatcher), body)
	if r.Status != 201 {
		t.Fatalf("получено %d %s", r.Status, r.Raw)
	}
	if got.VehicleID != 7 || got.TripID != 3 || !got.Timestamp.Equal(when) || got.Latitude != 50.45 || got.Longitude != 30.52 || got.SpeedKmh != 41.5 {
		t.Fatalf("сервису передано %+v", got)
	}
}

func TestTelemetryEndpointsRequireTokenAndMapErrors(t *testing.T) {
	h, tokens := newAPI(t, withTelemetry(stubTelemetry{record: func(domain.TelemetryPoint) error { return domain.ErrInvalidInput }}))
	if r := do(t, h, "POST", "/api/v1/telemetry", "", map[string]any{}); r.Status != 401 {
		t.Errorf("без токена: %d", r.Status)
	}
	if r := do(t, h, "POST", "/api/v1/telemetry", token(t, tokens, 1, domain.RoleDispatcher), map[string]any{"vehicle_id": 1}); r.Status != 400 {
		t.Errorf("ошибка валидации сервиса должна дать 400, получено %d", r.Status)
	}
}

func TestTelemetryHistoryParsesQuery(t *testing.T) {
	var vehicle int64
	var from, to time.Time
	var limit int
	h, tokens := newAPI(t, withTelemetry(stubTelemetry{history: func(v int64, a, b time.Time, l int) ([]domain.TelemetryPoint, error) {
		vehicle, from, to, limit = v, a, b, l
		return []domain.TelemetryPoint{{VehicleID: v, Timestamp: a, Latitude: 50, Longitude: 30, SpeedKmh: 10}}, nil
	}}))
	tok := token(t, tokens, 1, domain.RoleDispatcher)
	r := do(t, h, "GET", "/api/v1/vehicles/7/telemetry?from=2026-10-01T08:00:00Z&to=2026-10-01T09:00:00Z&limit=50", tok, nil)
	if r.Status != 200 || !strings.Contains(r.Raw, `"speed_kmh":10`) {
		t.Fatalf("получено %d %s", r.Status, r.Raw)
	}
	if vehicle != 7 || !from.Equal(when) || !to.Equal(when.Add(time.Hour)) || limit != 50 {
		t.Fatalf("сервису передано %d %v %v %d", vehicle, from, to, limit)
	}
	for _, q := range []string{"", "?from=2026-10-01T08:00:00Z", "?from=вчера&to=2026-10-01T09:00:00Z", "?from=2026-10-01T08:00:00Z&to=2026-10-01T09:00:00Z&limit=x"} {
		if r := do(t, h, "GET", "/api/v1/vehicles/7/telemetry"+q, tok, nil); r.Status != 400 {
			t.Errorf("запрос %q: ожидался 400, получено %d", q, r.Status)
		}
	}
}

// ---- журналы рейсов ----

func TestJournalEndpoints(t *testing.T) {
	var event domain.JournalEvent
	var tripID int64
	h, tokens := newAPI(t, withJournals(stubJournals{
		create: func(id int64) (domain.TripJournal, error) { return domain.TripJournal{TripID: id}, nil },
		addEvent: func(id int64, e domain.JournalEvent) (domain.TripJournal, error) {
			tripID, event = id, e
			return domain.TripJournal{TripID: id, Events: []domain.JournalEvent{e}}, nil
		},
		delete: func(int64) error { return nil },
	}))
	disp, admin := token(t, tokens, 2, domain.RoleDispatcher), token(t, tokens, 1, domain.RoleAdmin)

	if r := do(t, h, "POST", "/api/v1/trips/5/journal", disp, nil); r.Status != 201 || r.Body["trip_id"] != float64(5) {
		t.Fatalf("создание журнала: %d %s", r.Status, r.Raw)
	}
	r := do(t, h, "POST", "/api/v1/trips/5/journal/events", disp, map[string]any{"at": "2026-10-01T08:05:00Z", "type": "DELAY_DETECTED", "delay_seconds": 420})
	if r.Status != 201 || !strings.Contains(r.Raw, `"delay_seconds":420`) {
		t.Fatalf("добавление события: %d %s", r.Status, r.Raw)
	}
	if tripID != 5 || event.Type != domain.EventDelayDetected || event.DelaySeconds != 420 || !event.At.Equal(when.Add(5*time.Minute)) {
		t.Fatalf("сервису передано %d %+v", tripID, event)
	}
	if r := do(t, h, "DELETE", "/api/v1/trips/5/journal", disp, nil); r.Status != 403 {
		t.Errorf("диспетчер не может удалять журнал: %d", r.Status)
	}
	if r := do(t, h, "DELETE", "/api/v1/trips/5/journal", admin, nil); r.Status != 204 {
		t.Errorf("администратор удаляет журнал: %d", r.Status)
	}
}

// ---- карточки инцидентов ----

func TestCaseEndpointsUseUserFromToken(t *testing.T) {
	var createdBy, commentAuthor, closedBy int64
	var title *string
	var tags *[]string
	c := domain.IncidentCase{IncidentID: 9, Title: "Задержка рейса", Tags: []string{"delay"}, Status: domain.CaseOpen, Comments: []domain.CaseComment{}}
	h, tokens := newAPI(t, withCases(stubCases{
		create: func(i, u int64, ti string, tg []string) (domain.IncidentCase, error) { createdBy = u; return c, nil },
		update: func(i int64, ti *string, tg *[]string) (domain.IncidentCase, error) {
			title, tags = ti, tg
			return c, nil
		},
		addComment: func(i, a int64, text string) (domain.IncidentCase, error) { commentAuthor = a; return c, nil },
		close: func(i, u int64) (domain.IncidentCase, error) {
			closedBy = u
			c.Status = domain.CaseClosed
			return c, nil
		},
	}))
	tok := token(t, tokens, 77, domain.RoleDispatcher)

	if r := do(t, h, "POST", "/api/v1/incidents/9/case", tok, map[string]any{"title": "Задержка рейса", "tags": []string{"delay"}}); r.Status != 201 || createdBy != 77 {
		t.Fatalf("создание: %d %s, автор=%d", r.Status, r.Raw, createdBy)
	}
	if r := do(t, h, "POST", "/api/v1/incidents/9/case/comments", tok, map[string]any{"text": "Водитель на связи"}); r.Status != 201 || commentAuthor != 77 {
		t.Fatalf("комментарий: %d %s, автор=%d", r.Status, r.Raw, commentAuthor)
	}
	if r := do(t, h, "POST", "/api/v1/incidents/9/case/close", tok, nil); r.Status != 200 || closedBy != 77 || r.Body["status"] != "CLOSED" {
		t.Fatalf("закрытие: %d %s, кто=%d", r.Status, r.Raw, closedBy)
	}
	// PATCH: переданные поля доходят как указатели, непереданные — nil
	if r := do(t, h, "PATCH", "/api/v1/incidents/9/case", tok, map[string]any{"title": "Новый заголовок"}); r.Status != 200 || title == nil || *title != "Новый заголовок" || tags != nil {
		t.Fatalf("PATCH только title: %d %s title=%v tags=%v", r.Status, r.Raw, title, tags)
	}
	if r := do(t, h, "PATCH", "/api/v1/incidents/9/case", tok, map[string]any{"tags": []string{}}); r.Status != 200 || tags == nil || len(*tags) != 0 {
		t.Fatalf("PATCH пустой список тегов должен дойти как пустой срез: %d %s", r.Status, r.Raw)
	}
	if r := do(t, h, "DELETE", "/api/v1/incidents/9/case", tok, nil); r.Status != 403 {
		t.Errorf("диспетчер не может удалять карточку: %d", r.Status)
	}
}

// ---- остановки и состав маршрутов ----

func TestStopPointNearIsPublicAndParsesQuery(t *testing.T) {
	var lat, lon float64
	var radius, limit int
	h, _ := newAPI(t, withGeo(stubGeo{near: func(a, b float64, r, l int) ([]domain.NearStop, error) {
		lat, lon, radius, limit = a, b, r, l
		return []domain.NearStop{{StopPoint: domain.StopPoint{StopID: 4, Name: "Парк", RouteIDs: []int64{1}}, DistanceM: 55.6}}, nil
	}}))
	r := do(t, h, "GET", "/api/v1/stop-points/near?lat=50.45&lon=30.52&radius=300&limit=3", "", nil)
	if r.Status != 200 || !strings.Contains(r.Raw, `"distance_m":55.6`) || !strings.Contains(r.Raw, `"stop_id":4`) {
		t.Fatalf("получено %d %s", r.Status, r.Raw)
	}
	if lat != 50.45 || lon != 30.52 || radius != 300 || limit != 3 {
		t.Fatalf("сервису передано %v %v %d %d", lat, lon, radius, limit)
	}
	for _, q := range []string{"", "?lat=50.45", "?lat=abc&lon=30", "?lat=50&lon=30&radius=x"} {
		if r := do(t, h, "GET", "/api/v1/stop-points/near"+q, "", nil); r.Status != 400 {
			t.Errorf("запрос %q: ожидался 400, получено %d", q, r.Status)
		}
	}
}

func TestStopPointAdminEndpointsAndAttributeNumbers(t *testing.T) {
	var attrs map[string]any
	h, tokens := newAPI(t, withGeo(stubGeo{
		createStop: func(id int64, a map[string]any) (domain.StopPoint, error) {
			attrs = a
			return domain.StopPoint{StopID: id, Name: "Вокзал", RouteIDs: []int64{}, Attributes: a}, nil
		},
		getStop: func(id int64) (domain.StopPoint, error) {
			return domain.StopPoint{StopID: id, Name: "Вокзал", RouteIDs: []int64{}}, nil
		},
	}))
	body := map[string]any{"stop_id": 2, "attributes": map[string]any{"shelter": true, "bench_count": 2, "width": 2.5, "surface": "asphalt"}}
	if r := do(t, h, "POST", "/api/v1/stop-points", token(t, tokens, 2, domain.RoleDispatcher), body); r.Status != 403 {
		t.Errorf("диспетчер не может создавать: %d", r.Status)
	}
	if r := do(t, h, "POST", "/api/v1/stop-points", token(t, tokens, 1, domain.RoleAdmin), body); r.Status != 201 {
		t.Fatalf("создание: %d %s", r.Status, r.Raw)
	}
	if v, ok := attrs["bench_count"].(int64); !ok || v != 2 {
		t.Errorf("целое число из JSON должно дойти как int64, получено %T %v", attrs["bench_count"], attrs["bench_count"])
	}
	if v, ok := attrs["width"].(float64); !ok || v != 2.5 {
		t.Errorf("дробное должно остаться float64: %T %v", attrs["width"], attrs["width"])
	}
	if r := do(t, h, "GET", "/api/v1/stop-points/2", "", nil); r.Status != 200 {
		t.Errorf("чтение открытое: %d", r.Status)
	}
}

func TestSetRouteStopsParsesBodyAndIsAdminOnly(t *testing.T) {
	var routeID int64
	var refs []domain.RouteStopRef
	h, tokens := newAPI(t, withGeo(stubGeo{setStops: func(id int64, r []domain.RouteStopRef) (domain.RouteProfile, error) {
		routeID, refs = id, r
		return domain.RouteProfile{RouteID: id, Stops: r}, nil
	}}))
	body := map[string]any{"stops": []map[string]any{{"stop_id": 11, "seq": 1}, {"stop_id": 12, "seq": 2}}}
	if r := do(t, h, "PUT", "/api/v1/route-profiles/3/stops", token(t, tokens, 2, domain.RoleDispatcher), body); r.Status != 403 {
		t.Errorf("диспетчер: %d", r.Status)
	}
	r := do(t, h, "PUT", "/api/v1/route-profiles/3/stops", token(t, tokens, 1, domain.RoleAdmin), body)
	if r.Status != 200 || !strings.Contains(r.Raw, `"seq":2`) {
		t.Fatalf("получено %d %s", r.Status, r.Raw)
	}
	if routeID != 3 || len(refs) != 2 || refs[0].StopID != 11 || refs[1].Seq != 2 {
		t.Fatalf("сервису передано %d %+v", routeID, refs)
	}
	if r := do(t, h, "PUT", "/api/v1/route-profiles/3/stops", token(t, tokens, 1, domain.RoleAdmin), map[string]any{"stop": 1}); r.Status != 400 {
		t.Errorf("неизвестное поле: %d", r.Status)
	}
}
