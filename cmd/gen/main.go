// Генератор данных для ЛР3. Детерминирован: один seed -> одни и те же данные.
//
//	go run ./cmd/gen --mode dev  --seed 42
//	go run ./cmd/gen --mode load --seed 42
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"math/big"
	"math/rand"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type config struct {
	routes, stops, vehicles, users, trips int
	minStops, maxStops                    int
}

var modes = map[string]config{
	"dev":  {routes: 20, stops: 300, vehicles: 200, users: 10, trips: 100_000, minStops: 5, maxStops: 15},
	"load": {routes: 50, stops: 1000, vehicles: 500, users: 20, trips: 3_000_000, minStops: 8, maxStops: 25},
}

// Фиксированное "сейчас", чтобы данные не зависели от времени запуска.
var now = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

const historyDays = 730

// Доля поездок по часам суток: утренний и вечерний пики, ночью почти пусто.
var hourWeights = [24]float64{
	0.2, 0.1, 0.1, 0.1, 0.3, 1, 3, 6, 8, 5, 4, 4,
	4, 4, 4, 5, 6, 8, 7, 4, 2, 1.5, 1, 0.5,
}

type trip struct {
	route, vehicle int64
	ps, pe         time.Time
	as, ae         pgtype.Timestamptz
	status         string
	created        time.Time
	delay          float64 // минуты, нужно только для генерации инцидентов
}

type incident struct {
	trip     int64
	typ      string
	status   string
	desc     string
	created  time.Time
	resolved pgtype.Timestamptz
	by       pgtype.Int8
}

type gen struct {
	r       *rand.Rand
	cfg     config
	hourCum [24]float64
}

func main() {
	mode := flag.String("mode", "dev", "dev | load")
	seed := flag.Int64("seed", 42, "seed генератора")
	dsn := flag.String("dsn", env("LAB3_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/trans-pulse_lab3?sslmode=disable"), "строка подключения")
	flag.Parse()

	cfg, ok := modes[*mode]
	if !ok {
		log.Fatalf("неизвестный режим %q", *mode)
	}

	g := newGen(cfg, *seed)
	start := time.Now()

	routes := g.routes()
	stopsRows := g.stops()
	routeStops := g.routeStops()
	vehicles := g.vehicles()
	users := g.users()
	trips, incidents := g.trips(vehicles.status, routes.bad, vehicles.bad)
	log.Printf("сгенерировано за %v: trips=%d incidents=%d", time.Since(start).Round(time.Millisecond), len(trips), len(incidents))

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, *dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(ctx)

	log.Printf("очистка таблиц в %s", conn.Config().Database)
	if _, err := conn.Exec(ctx, `TRUNCATE incidents, trips, users, vehicles, route_stops, stops, routes RESTART IDENTITY`); err != nil {
		log.Fatal(err)
	}

	// id у таблиц GENERATED ALWAYS: после TRUNCATE RESTART IDENTITY они выдаются
	// подряд 1..N в порядке COPY, поэтому внешние ключи считаем по индексу строки.
	loadStart := time.Now()
	copyTo(ctx, conn, "routes", []string{"number", "name", "created_at"}, routes.src)
	copyTo(ctx, conn, "stops", []string{"name", "latitude", "longitude", "created_at"}, stopsRows)
	copyTo(ctx, conn, "route_stops", []string{"route_id", "stop_id", "sequence_no"}, routeStops)
	copyTo(ctx, conn, "vehicles", []string{"fleet_number", "model", "status", "created_at"}, vehicles.src)
	copyTo(ctx, conn, "users", []string{"login", "password_hash", "role", "created_at"}, users)
	copyTo(ctx, conn, "trips", []string{"route_id", "vehicle_id", "planned_start", "planned_end", "actual_start", "actual_end", "status", "created_at"}, &tripSrc{t: trips})
	copyTo(ctx, conn, "incidents", []string{"trip_id", "type", "status", "description", "created_at", "resolved_at", "resolved_by_user_id"}, &incSrc{t: incidents})
	log.Printf("загрузка COPY: %v", time.Since(loadStart).Round(time.Millisecond))

	if _, err := conn.Exec(ctx, `ANALYZE`); err != nil {
		log.Fatal(err)
	}
	var nTrips, maxTrip, nInc, maxInc int64
	if err := conn.QueryRow(ctx, `SELECT (SELECT count(*) FROM trips), (SELECT max(id) FROM trips), (SELECT count(*) FROM incidents), (SELECT max(id) FROM incidents)`).Scan(&nTrips, &maxTrip, &nInc, &maxInc); err != nil {
		log.Fatal(err)
	}
	if nTrips != maxTrip || nInc != maxInc {
		log.Fatalf("id не последовательны: trips %d/%d incidents %d/%d", nTrips, maxTrip, nInc, maxInc)
	}
	log.Printf("готово: trips=%d incidents=%d, всего %v", nTrips, nInc, time.Since(start).Round(time.Millisecond))
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func newGen(cfg config, seed int64) *gen {
	g := &gen{r: rand.New(rand.NewSource(seed)), cfg: cfg}
	var sum float64
	for h, w := range hourWeights {
		sum += w
		g.hourCum[h] = sum
	}
	for h := range g.hourCum {
		g.hourCum[h] /= sum
	}
	return g
}

func (g *gen) hour() int {
	u := g.r.Float64()
	for h, c := range g.hourCum {
		if u <= c {
			return h
		}
	}
	return 23
}

// bad возвращает "коэффициент проблемности" 1..5: у большинства около 1, у немногих высокий.
func (g *gen) bad() float64 { return 1 + 4*math.Pow(g.r.Float64(), 4) }

func num(v float64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(int64(math.Round(v * 1e6))), Exp: -6, Valid: true}
}

type routeData struct {
	src pgx.CopyFromSource
	bad []float64
}

func (g *gen) routes() routeData {
	rows := make([][]any, g.cfg.routes)
	bad := make([]float64, g.cfg.routes)
	for i := range rows {
		rows[i] = []any{fmt.Sprintf("%d", i+1), fmt.Sprintf("Маршрут %d", i+1), now.AddDate(-3, 0, 0)}
		bad[i] = g.bad()
	}
	return routeData{pgx.CopyFromRows(rows), bad}
}

func (g *gen) stops() pgx.CopyFromSource {
	rows := make([][]any, g.cfg.stops)
	for i := range rows {
		lat := 50.35 + g.r.Float64()*0.2
		lon := 30.42 + g.r.Float64()*0.2
		rows[i] = []any{fmt.Sprintf("Остановка %d", i+1), num(lat), num(lon), now.AddDate(-3, 0, 0)}
	}
	return pgx.CopyFromRows(rows)
}

// У маршрутов разное число остановок; популярные остановки (пересадочные узлы)
// входят в много маршрутов, редкие — в один или ни в один.
func (g *gen) routeStops() pgx.CopyFromSource {
	zipf := rand.NewZipf(g.r, 1.2, 5, uint64(g.cfg.stops-1))
	var rows [][]any
	for route := 1; route <= g.cfg.routes; route++ {
		k := g.cfg.minStops + g.r.Intn(g.cfg.maxStops-g.cfg.minStops+1)
		used := map[uint64]bool{}
		for seq := 1; seq <= k; {
			s := zipf.Uint64()
			if used[s] {
				continue
			}
			used[s] = true
			rows = append(rows, []any{int64(route), int64(s + 1), int16(seq)})
			seq++
		}
	}
	return pgx.CopyFromRows(rows)
}

type vehicleData struct {
	src    pgx.CopyFromSource
	status []string
	bad    []float64
}

var models = []string{"MAN Lion's City", "Mercedes Citaro", "ЛиАЗ-5292", "Volvo 7900", "МАЗ-203"}

func (g *gen) vehicles() vehicleData {
	rows := make([][]any, g.cfg.vehicles)
	status := make([]string, g.cfg.vehicles)
	bad := make([]float64, g.cfg.vehicles)
	for i := range rows {
		switch u := g.r.Float64(); {
		case u < 0.80:
			status[i] = "IN_SERVICE"
		case u < 0.90:
			status[i] = "OUT_OF_SERVICE"
		case u < 0.95:
			status[i] = "REGISTERED"
		default:
			status[i] = "RETIRED"
		}
		bad[i] = g.bad()
		rows[i] = []any{fmt.Sprintf("B%04d", i+1), models[g.r.Intn(len(models))], status[i], now.AddDate(-2, 0, 0)}
	}
	return vehicleData{pgx.CopyFromRows(rows), status, bad}
}

func (g *gen) users() pgx.CopyFromSource {
	rows := make([][]any, g.cfg.users)
	for i := range rows {
		role := "DISPATCHER"
		if i < 2 {
			role = "ADMIN"
		}
		rows[i] = []any{fmt.Sprintf("user%02d", i+1), fmt.Sprintf("demo-hash-%d", i+1), role, now.AddDate(-2, 0, 0)}
	}
	return pgx.CopyFromRows(rows)
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func (g *gen) trips(vStatus []string, routeBad, vehBad []float64) ([]trip, []incident) {
	routeZipf := rand.NewZipf(g.r, 1.3, 2, uint64(g.cfg.routes-1))

	var eligible, inService []int64 // автобусы, которые могли ездить / ездят сейчас
	for i, s := range vStatus {
		if s != "REGISTERED" {
			eligible = append(eligible, int64(i+1))
		}
		if s == "IN_SERVICE" {
			inService = append(inService, int64(i+1))
		}
	}
	vehZipf := rand.NewZipf(g.r, 1.1, 30, uint64(len(eligible)-1))
	vehServZipf := rand.NewZipf(g.r, 1.1, 30, uint64(len(inService)-1))
	dispatchers := uint64(g.cfg.users - 2)
	userZipf := rand.NewZipf(g.r, 1.5, 1, dispatchers-1)

	routeBase := make([]int, g.cfg.routes)
	for i := range routeBase {
		routeBase[i] = 25 + g.r.Intn(46)
	}

	trips := make([]trip, 0, g.cfg.trips)
	var incs []incident
	addInc := func(tripID int64, typ string, created time.Time, open bool) {
		in := incident{trip: tripID, typ: typ, status: "OPEN", created: created}
		if typ == "DELAY" {
			in.desc = "Trip exceeds the configured delay threshold"
		} else {
			in.desc = "No recent telemetry received"
		}
		if !open {
			in.status = "RESOLVED"
			mins := 5 * math.Exp(g.r.NormFloat64()*0.9) // медиана 5 минут, длинный хвост
			in.resolved = ts(created.Add(time.Duration(mins*60+60) * time.Second))
			in.by = pgtype.Int8{Int64: int64(userZipf.Uint64()) + 3, Valid: true} // диспетчеры идут после 2 админов
		}
		incs = append(incs, in)
	}

	// 1. Активные рейсы: у ~60% автобусов в эксплуатации идёт ровно один рейс.
	for _, v := range inService {
		if g.r.Float64() >= 0.6 {
			continue
		}
		route := int64(routeZipf.Uint64())
		ps := now.Add(-time.Duration(10+g.r.Intn(50)) * time.Minute)
		dur := routeBase[route] + g.r.Intn(15)
		t := trip{
			route: route + 1, vehicle: v, ps: ps, pe: ps.Add(time.Duration(dur) * time.Minute),
			as: ts(ps.Add(time.Duration(1+g.r.Intn(3)) * time.Minute)), status: "IN_PROGRESS",
			created: ps.Add(-time.Duration(1+g.r.Intn(48)) * time.Hour),
		}
		trips = append(trips, t)
		id := int64(len(trips))
		if g.r.Float64() < 0.25 {
			addInc(id, "DELAY", ps.Add(6*time.Minute), true)
		}
		if g.r.Float64() < 0.10 {
			addInc(id, "NO_TELEMETRY", ps.Add(5*time.Minute), true)
		}
	}

	// 2. Запланированные рейсы на ближайшую неделю (~1%).
	for i := 0; i < g.cfg.trips/100; i++ {
		route := int64(routeZipf.Uint64())
		day := g.r.Intn(7)
		ps := now.Add(time.Duration(day*24+g.hour())*time.Hour + time.Duration(g.r.Intn(60))*time.Minute)
		trips = append(trips, trip{
			route: route + 1, vehicle: inService[vehServZipf.Uint64()], ps: ps,
			pe:     ps.Add(time.Duration(routeBase[route]+g.r.Intn(15)) * time.Minute),
			status: "PLANNED", created: now.Add(-time.Duration(g.r.Intn(72)) * time.Hour),
		})
	}

	// 3. История: плотность растёт к настоящему моменту, статусы COMPLETED/CANCELLED.
	for len(trips) < g.cfg.trips {
		day := int(historyDays * math.Sqrt(g.r.Float64()))
		ps := now.AddDate(0, 0, -historyDays+day).Add(time.Duration(g.hour())*time.Hour + time.Duration(g.r.Intn(3600))*time.Second).Truncate(time.Minute)
		if ps.After(now.Add(-2 * time.Hour)) {
			continue
		}
		route := int64(routeZipf.Uint64())
		vi := vehZipf.Uint64()
		veh := eligible[vi]
		dur := routeBase[route] + g.r.Intn(15)
		t := trip{
			route: route + 1, vehicle: veh, ps: ps, pe: ps.Add(time.Duration(dur) * time.Minute),
			created: ps.Add(-time.Duration(1+g.r.Intn(48)) * time.Hour),
		}
		rb, vb := routeBad[route], vehBad[veh-1]
		if g.r.Float64() < math.Min(0.25, 0.04*rb) {
			t.status = "CANCELLED"
			trips = append(trips, t)
			continue
		}
		t.status = "COMPLETED"
		switch u := g.r.Float64(); {
		case u < 0.60:
			t.delay = g.r.Float64() * 2
		case u < 0.97:
			t.delay = -3 * math.Sqrt(rb*vb) * math.Log(1-g.r.Float64())
		default:
			t.delay = 20 + g.r.Float64()*40
		}
		start := ps.Add(time.Duration(t.delay * float64(time.Minute)))
		end := start.Add(time.Duration(dur+g.r.Intn(6)) * time.Minute)
		t.as, t.ae = ts(start), ts(end)
		trips = append(trips, t)
		id := int64(len(trips))

		span := int(end.Sub(start) / time.Minute)
		if t.delay > 5 && g.r.Float64() < 0.5 {
			addInc(id, "DELAY", start.Add(time.Duration(g.r.Intn(span))*time.Minute), g.r.Float64() < 0.005)
		}
		if g.r.Float64() < 0.01*vb {
			addInc(id, "NO_TELEMETRY", start.Add(time.Duration(g.r.Intn(span))*time.Minute), g.r.Float64() < 0.005)
		}
	}
	return trips, incs
}

func copyTo(ctx context.Context, conn *pgx.Conn, table string, cols []string, src pgx.CopyFromSource) {
	n, err := conn.CopyFrom(ctx, pgx.Identifier{table}, cols, src)
	if err != nil {
		log.Fatalf("COPY %s: %v", table, err)
	}
	log.Printf("  %-12s %d строк", table, n)
}

type tripSrc struct {
	t []trip
	i int
}

func (s *tripSrc) Next() bool { s.i++; return s.i <= len(s.t) }
func (s *tripSrc) Values() ([]any, error) {
	t := s.t[s.i-1]
	return []any{t.route, t.vehicle, t.ps, t.pe, t.as, t.ae, t.status, t.created}, nil
}
func (s *tripSrc) Err() error { return nil }

type incSrc struct {
	t []incident
	i int
}

func (s *incSrc) Next() bool { s.i++; return s.i <= len(s.t) }
func (s *incSrc) Values() ([]any, error) {
	t := s.t[s.i-1]
	return []any{t.trip, t.typ, t.status, t.desc, t.created, t.resolved, t.by}, nil
}
func (s *incSrc) Err() error { return nil }
