package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/service"
)

type telemetryEnv struct {
	repo *fakeTelemetry
	bus  domain.Vehicle
	svc  *service.TelemetryService
}

func newTelemetryEnv() telemetryEnv {
	vehicles := newFakeVehicles()
	e := telemetryEnv{repo: &fakeTelemetry{}, bus: vehicles.add("B1", domain.VehicleInService)}
	e.svc = service.NewTelemetryService(e.repo, vehicles, func() time.Time { return t0 })
	return e
}

func (e telemetryEnv) point() domain.TelemetryPoint {
	return domain.TelemetryPoint{VehicleID: e.bus.ID, Timestamp: t0, Latitude: 50.45, Longitude: 30.52, SpeedKmh: 40}
}

func TestTelemetryRecordStoresValidPoint(t *testing.T) {
	e := newTelemetryEnv()
	if err := e.svc.Record(ctx, e.point()); err != nil || len(e.repo.points) != 1 {
		t.Fatalf("Record: %v, сохранено %d", err, len(e.repo.points))
	}
}

func TestTelemetryRecordRejectsInvalidPoints(t *testing.T) {
	cases := map[string]func(p *domain.TelemetryPoint){
		"широта больше 90 (BR-10)":      func(p *domain.TelemetryPoint) { p.Latitude = 90.5 },
		"широта меньше -90":             func(p *domain.TelemetryPoint) { p.Latitude = -91 },
		"долгота больше 180 (BR-11)":    func(p *domain.TelemetryPoint) { p.Longitude = 181 },
		"нет времени измерения (BR-12)": func(p *domain.TelemetryPoint) { p.Timestamp = time.Time{} },
		"отрицательная скорость":        func(p *domain.TelemetryPoint) { p.SpeedKmh = -1 },
		"скорость выше 250":             func(p *domain.TelemetryPoint) { p.SpeedKmh = 251 },
		"время из будущего":             func(p *domain.TelemetryPoint) { p.Timestamp = t0.Add(10 * time.Minute) },
	}
	for name, mutate := range cases {
		e := newTelemetryEnv()
		p := e.point()
		mutate(&p)
		if err := e.svc.Record(ctx, p); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: ожидалось ErrInvalidInput, получено %v", name, err)
		}
		if len(e.repo.points) != 0 {
			t.Errorf("%s: некорректная точка не должна сохраняться", name)
		}
	}
}

func TestTelemetryRecordRequiresExistingVehicle(t *testing.T) {
	e := newTelemetryEnv()
	p := e.point()
	p.VehicleID = 999
	if err := e.svc.Record(ctx, p); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("BR-09: ожидалось ErrInvalidInput, получено %v", err)
	}
}

func TestTelemetryHistoryValidatesPeriod(t *testing.T) {
	e := newTelemetryEnv()
	e.svc.Record(ctx, e.point())

	got, err := e.svc.History(ctx, e.bus.ID, t0.Add(-time.Hour), t0.Add(time.Hour), 0)
	if err != nil || len(got) != 1 {
		t.Fatalf("History: %+v, %v", got, err)
	}
	if _, err := e.svc.History(ctx, e.bus.ID, t0, t0, 10); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("пустой период: %v", err)
	}
	if _, err := e.svc.History(ctx, e.bus.ID, t0, t0.Add(-time.Hour), 10); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("конец раньше начала: %v", err)
	}
	if _, err := e.svc.History(ctx, e.bus.ID, t0, t0.Add(25*time.Hour), 10); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("период больше 24 часов: %v", err)
	}
	if _, err := e.svc.History(ctx, 999, t0, t0.Add(time.Hour), 10); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("несуществующий автобус: %v", err)
	}
}
