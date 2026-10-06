// Сценарии ЛР3 на живой БД: гонка, диагностика ожидания блокировки, deadlock, сравнение загрузки.
//
//	go run ./cmd/lab3 race
//	go run ./cmd/lab3 deadlock
//	go run ./cmd/lab3 loadbench
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var dsn = envOr("LAB3_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/trans-pulse_lab3?sslmode=disable")

var (
	t0   = time.Now()
	logM sync.Mutex
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("использование: lab3 race|deadlock|loadbench")
	}
	ctx := context.Background()
	switch os.Args[1] {
	case "race":
		race(ctx)
	case "deadlock":
		deadlock(ctx)
	case "loadbench":
		loadbench(ctx)
	default:
		log.Fatalf("неизвестная команда %q", os.Args[1])
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func logf(format string, args ...any) {
	logM.Lock()
	defer logM.Unlock()
	fmt.Printf("[%5.2fs] %s\n", time.Since(t0).Seconds(), fmt.Sprintf(format, args...))
}

func connect(ctx context.Context) *pgx.Conn {
	c, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	return c
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// ---------- Гонка: BR-07 "один автобус — один активный рейс" ----------

const uqIndex = `CREATE UNIQUE INDEX IF NOT EXISTS uq_trips_one_active_per_vehicle ON trips(vehicle_id) WHERE status = 'IN_PROGRESS'`

func race(ctx context.Context) {
	admin := connect(ctx)
	defer admin.Close(ctx)

	var vehicle int64
	var tripIDs []int64
	must(admin.QueryRow(ctx, `
		SELECT vehicle_id, (array_agg(id ORDER BY id))[1:2]
		FROM trips
		WHERE status = 'PLANNED'
		  AND vehicle_id IN (SELECT v.id FROM vehicles v WHERE v.status = 'IN_SERVICE'
		       AND NOT EXISTS (SELECT 1 FROM trips x WHERE x.vehicle_id = v.id AND x.status = 'IN_PROGRESS'))
		GROUP BY vehicle_id HAVING count(*) >= 2 ORDER BY vehicle_id LIMIT 1`).Scan(&vehicle, &tripIDs))
	fmt.Printf("Автобус %d, два запланированных рейса: %v\n", vehicle, tripIDs)
	defer admin.Exec(ctx, uqIndex) // в любом случае вернуть индекс

	reset := func() {
		_, err := admin.Exec(ctx, `UPDATE trips SET status='PLANNED', actual_start=NULL WHERE id = ANY($1)`, tripIDs)
		must(err)
	}
	active := func() int {
		var n int
		must(admin.QueryRow(ctx, `SELECT count(*) FROM trips WHERE vehicle_id=$1 AND status='IN_PROGRESS'`, vehicle).Scan(&n))
		return n
	}

	runVariant := func(title string, lock bool) {
		fmt.Printf("\n=== %s\n", title)
		t0 = time.Now()
		var wg sync.WaitGroup
		for i, id := range tripIDs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				dispatcher(ctx, fmt.Sprintf("диспетчер-%d", i+1), vehicle, id, lock)
			}()
		}
		time.Sleep(500 * time.Millisecond)
		showWaits(ctx, admin)
		wg.Wait()
		n := active()
		verdict := "правило BR-07 соблюдено"
		if n > 1 {
			verdict = "ПРАВИЛО НАРУШЕНО"
		}
		fmt.Printf("Итог: активных рейсов у автобуса %d — %d (%s)\n", vehicle, n, verdict)
		reset()
	}

	runVariant("Вариант B: проверка-потом-запись, индекс uq_trips_one_active_per_vehicle ЕСТЬ", false)

	_, err := admin.Exec(ctx, `DROP INDEX uq_trips_one_active_per_vehicle`)
	must(err)
	runVariant("Вариант A: проверка-потом-запись, индекса НЕТ (защиты нет)", false)
	runVariant("Вариант C: SELECT ... FOR UPDATE по автобусу, индекса НЕТ", true)
	_, err = admin.Exec(ctx, uqIndex)
	must(err)
	fmt.Println("\nИндекс uq_trips_one_active_per_vehicle восстановлен.")
}

// dispatcher запускает рейс: проверка, "время на раздумья" (1 с), UPDATE, COMMIT.
func dispatcher(ctx context.Context, name string, vehicle, trip int64, lock bool) {
	c := connect(ctx)
	defer c.Close(ctx)
	tx, err := c.Begin(ctx)
	must(err)
	defer tx.Rollback(ctx)

	if lock {
		logf("%s: SELECT ... FROM vehicles WHERE id=%d FOR UPDATE", name, vehicle)
		_, err := tx.Exec(ctx, `SELECT id FROM vehicles WHERE id=$1 FOR UPDATE`, vehicle)
		must(err)
		logf("%s: блокировка автобуса получена", name)
	}
	var n int
	must(tx.QueryRow(ctx, `SELECT count(*) FROM trips WHERE vehicle_id=$1 AND status='IN_PROGRESS'`, vehicle).Scan(&n))
	logf("%s: проверка — активных рейсов у автобуса: %d", name, n)
	if n > 0 {
		logf("%s: ОТКАЗ — у автобуса уже есть активный рейс, ROLLBACK", name)
		return
	}
	time.Sleep(time.Second) // имитация работы приложения между проверкой и записью
	logf("%s: UPDATE рейса %d -> IN_PROGRESS", name, trip)
	if _, err := tx.Exec(ctx, `UPDATE trips SET status='IN_PROGRESS', actual_start=CURRENT_TIMESTAMP WHERE id=$1 AND status='PLANNED'`, trip); err != nil {
		reportErr(name, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		reportErr(name, err)
		return
	}
	logf("%s: COMMIT", name)
}

func reportErr(name string, err error) {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		logf("%s: ОШИБКА SQLSTATE %s (%s), ROLLBACK", name, pe.Code, pe.ConstraintName)
		if pe.Detail != "" {
			logf("%s:   %s", name, pe.Detail)
		}
		return
	}
	logf("%s: ОШИБКА %v", name, err)
}

// showWaits — диагностика ожидания блокировки: кто кого блокирует.
func showWaits(ctx context.Context, c *pgx.Conn) {
	rows, err := c.Query(ctx, `
		SELECT a.pid, a.wait_event_type, a.wait_event, pg_blocking_pids(a.pid)::text, left(a.query, 70)
		FROM pg_stat_activity a
		WHERE a.datname = current_database() AND a.pid <> pg_backend_pid() AND cardinality(pg_blocking_pids(a.pid)) > 0`)
	must(err)
	defer rows.Close()
	found := false
	for rows.Next() {
		var pid int
		var wt, we, blockers, q *string
		must(rows.Scan(&pid, &wt, &we, &blockers, &q))
		found = true
		logf("ДИАГНОСТИКА pg_stat_activity: pid=%d ждёт %s/%s, заблокирован pid %s, запрос: %s", pid, deref(wt), deref(we), deref(blockers), deref(q))
	}
	rows.Close()
	if !found {
		logf("ДИАГНОСТИКА: никто никого не ждёт")
		return
	}
	lr, err := c.Query(ctx, `SELECT pid, locktype, mode, coalesce(relation::regclass::text, transactionid::text, '') FROM pg_locks WHERE NOT granted`)
	must(err)
	defer lr.Close()
	for lr.Next() {
		var pid int
		var lt, mode, obj string
		must(lr.Scan(&pid, &lt, &mode, &obj))
		logf("ДИАГНОСТИКА pg_locks (granted=false): pid=%d locktype=%s mode=%s object=%s", pid, lt, mode, obj)
	}
}

func deref(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}

// ---------- Deadlock ----------

func deadlock(ctx context.Context) {
	admin := connect(ctx)
	defer admin.Close(ctx)
	var a, b int64
	must(admin.QueryRow(ctx, `SELECT min(id), max(id) FROM (SELECT id FROM vehicles WHERE status='IN_SERVICE' ORDER BY id LIMIT 2) x`).Scan(&a, &b))

	run := func(title string, ordered bool) {
		fmt.Printf("\n=== %s (автобусы %d и %d)\n", title, a, b)
		t0 = time.Now()
		var wg sync.WaitGroup
		for i, pair := range [][2]int64{{a, b}, {b, a}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				transfer(ctx, fmt.Sprintf("T%d", i+1), pair[0], pair[1], ordered)
			}()
		}
		wg.Wait()
	}
	run("Без исправления: T1 блокирует A затем B, T2 — B затем A", false)

	out, _ := exec.Command("docker", "logs", "--tail", "40", "go_postgres").CombinedOutput()
	fmt.Println("\n--- Лог сервера PostgreSQL (строки про deadlock):")
	for _, l := range strings.Split(string(out), "\n") {
		if strings.Contains(l, "deadlock") || strings.Contains(l, "Process ") || strings.Contains(l, "CONTEXT") || strings.Contains(l, "STATEMENT") {
			fmt.Println(l)
		}
	}

	run("Исправление: оба берут блокировки в порядке возрастания id", true)
}

// transfer имитирует операцию, которой нужны обе строки автобусов (например, обмен рейсами).
func transfer(ctx context.Context, name string, first, second int64, ordered bool) {
	c := connect(ctx)
	defer c.Close(ctx)
	tx, err := c.Begin(ctx)
	must(err)
	defer tx.Rollback(ctx)

	lockOne := func(id int64) error {
		logf("%s: SELECT ... FROM vehicles WHERE id=%d FOR UPDATE", name, id)
		_, err := tx.Exec(ctx, `SELECT id FROM vehicles WHERE id=$1 FOR UPDATE`, id)
		return err
	}
	if ordered {
		if first > second {
			first, second = second, first
		}
	}
	if err := lockOne(first); err != nil {
		reportErr(name, err)
		return
	}
	time.Sleep(500 * time.Millisecond)
	if err := lockOne(second); err != nil {
		reportErr(name, err)
		return
	}
	logf("%s: обе строки заблокированы, COMMIT", name)
	must(tx.Commit(ctx))
}

// ---------- Сравнение стратегий загрузки ----------

const benchRows = 50_000

func loadbench(ctx context.Context) {
	c := connect(ctx)
	defer c.Close(ctx)
	base := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	row := func(i int) []any {
		ps := base.Add(time.Duration(i) * time.Minute)
		return []any{int64(i%50 + 1), int64(i%500 + 1), ps, ps.Add(40 * time.Minute), ps.Add(time.Minute), ps.Add(41 * time.Minute), "COMPLETED", ps.Add(-time.Hour)}
	}
	cols := []string{"route_id", "vehicle_id", "planned_start", "planned_end", "actual_start", "actual_end", "status", "created_at"}
	const ins = `INSERT INTO trips_bench (route_id, vehicle_id, planned_start, planned_end, actual_start, actual_end, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`

	strategies := []struct {
		name string
		run  func()
	}{
		{"INSERT по одной строке (autocommit)", func() {
			for i := 0; i < benchRows; i++ {
				_, err := c.Exec(ctx, ins, row(i)...)
				must(err)
			}
		}},
		{"INSERT пачками по 1000 в одной транзакции (pgx.Batch)", func() {
			tx, err := c.Begin(ctx)
			must(err)
			for start := 0; start < benchRows; start += 1000 {
				b := &pgx.Batch{}
				for i := start; i < start+1000 && i < benchRows; i++ {
					b.Queue(ins, row(i)...)
				}
				must(tx.SendBatch(ctx, b).Close())
			}
			must(tx.Commit(ctx))
		}},
		{"COPY (pgx.CopyFrom)", func() {
			rows := make([][]any, benchRows)
			for i := range rows {
				rows[i] = row(i)
			}
			_, err := c.CopyFrom(ctx, pgx.Identifier{"trips_bench"}, cols, pgx.CopyFromRows(rows))
			must(err)
		}},
	}

	fmt.Printf("Загрузка %d строк в trips_bench (без индексов и FK), 3 прогона на стратегию\n\n", benchRows)
	fmt.Printf("%-58s %10s %12s %12s\n", "стратегия", "медиана,с", "строк/с", "размер")
	for _, s := range strategies {
		var times []float64
		var size string
		for r := 0; r < 3; r++ {
			_, err := c.Exec(ctx, `DROP TABLE IF EXISTS trips_bench; CREATE TABLE trips_bench (LIKE trips INCLUDING DEFAULTS INCLUDING IDENTITY)`)
			must(err)
			start := time.Now()
			s.run()
			times = append(times, time.Since(start).Seconds())
			must(c.QueryRow(ctx, `SELECT pg_size_pretty(pg_total_relation_size('trips_bench'))`).Scan(&size))
		}
		sort.Float64s(times)
		fmt.Printf("%-58s %10.2f %12.0f %12s\n", s.name, times[1], benchRows/times[1], size)
	}
	_, err := c.Exec(ctx, `DROP TABLE trips_bench`)
	must(err)
}
