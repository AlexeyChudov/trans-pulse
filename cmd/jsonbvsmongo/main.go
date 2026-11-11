// Команда jsonbvsmongo сравнивает два способа хранить журнал событий рейса на ОДНИХ И ТЕХ ЖЕ данных:
//
//	A) PostgreSQL: таблица с колонкой jsonb (события — массив внутри строки) и GIN-индексом;
//	B) MongoDB: коллекция документов со встроенным массивом событий и составным индексом.
//
// Измеряются загрузка, размер на диске, чтение и добавление, поиск по равенству и по диапазону внутри массива,
// аналитика по всем событиям, а также защита данных (валидация). Результаты обоих хранилищ сверяются между собой.
//
// Запуск: set -a; . ./.env; set +a; go run ./cmd/jsonbvsmongo [-journals 5000 -events 100]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	mongorepo "github.com/AlexeyChudov/trans-pulse/internal/repository/mongo"
)

const (
	pgTable    = "exp_trip_journals"
	mongoDB    = "trans_pulse_experiment"
	mongoColl  = "trip_journals_exp"
	reads      = 200 // число точечных чтений и добавлений
	queryRuns  = 5
	targetSecs = 600 // «задержка ровно 10 минут» для поиска по равенству
	rangeSecs  = 850 // «задержка больше 850 с» для поиска по диапазону
)

var base = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

type event struct {
	At           time.Time `bson:"at" json:"-"`
	AtText       string    `bson:"-" json:"at"`
	Type         string    `bson:"type" json:"type"`
	StopID       int64     `bson:"stop_id,omitempty" json:"stop_id,omitempty"`
	DelaySeconds int       `bson:"delay_seconds,omitempty" json:"delay_seconds,omitempty"`
	Note         string    `bson:"note,omitempty" json:"note,omitempty"`
}

func genEvents(rng *rand.Rand, n int) []event {
	events := make([]event, 0, n)
	for i := 0; i < n; i++ {
		e := event{At: base.Add(time.Duration(i) * 30 * time.Second), Type: "ARRIVED_AT_STOP", StopID: int64(rng.Intn(200) + 1)}
		switch r := rng.Float64(); {
		case i == 0:
			e = event{At: e.At, Type: "DEPARTED"}
		case r < 0.70: // прибытие на остановку (значение по умолчанию)
		case r < 0.85:
			e = event{At: e.At, Type: "DELAY_DETECTED", DelaySeconds: 60 + 30*rng.Intn(29)} // 60…900 с
		case r < 0.90:
			e = event{At: e.At, Type: "TELEMETRY_LOST"}
		case r < 0.95:
			e = event{At: e.At, Type: "TELEMETRY_RESTORED"}
		default:
			e = event{At: e.At, Type: "NOTE", Note: "пробка на перекрёстке"}
		}
		e.AtText = e.At.Format(time.RFC3339)
		events = append(events, e)
	}
	return events
}

func median(d []time.Duration) time.Duration {
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	return d[len(d)/2]
}

func ms(d time.Duration) string { return fmt.Sprintf("%.2f", float64(d.Microseconds())/1000) }

func main() {
	journals := flag.Int("journals", 5000, "число журналов (рейсов)")
	eventsPer := flag.Int("events", 100, "событий в каждом журнале")
	flag.Parse()
	if err := run(*journals, *eventsPer); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

type row struct{ name, pg, mg string }

func run(journals, eventsPer int) error {
	ctx := context.Background()
	pgURL, mongoURI := os.Getenv("DATABASE_URL"), os.Getenv("MONGO_URI")
	if pgURL == "" || mongoURI == "" {
		return errors.New("задайте DATABASE_URL (PostgreSQL) и MONGO_URI (MongoDB)")
	}
	pg, err := pgx.Connect(ctx, pgURL)
	if err != nil {
		return err
	}
	defer pg.Close(ctx)
	mc, err := mongorepo.Connect(ctx, mongoURI)
	if err != nil {
		return err
	}
	defer mc.Disconnect(ctx)
	mdb := mc.Database(mongoDB)
	defer mdb.Drop(ctx)
	defer pg.Exec(ctx, "DROP TABLE IF EXISTS "+pgTable)
	_ = mdb.Drop(ctx)
	coll := mdb.Collection(mongoColl)

	if _, err := pg.Exec(ctx, "DROP TABLE IF EXISTS "+pgTable); err != nil {
		return err
	}
	if _, err := pg.Exec(ctx, `CREATE TABLE `+pgTable+` (trip_id bigint PRIMARY KEY, events jsonb NOT NULL, last_event_at timestamptz NOT NULL)`); err != nil {
		return err
	}
	// Индексы: jsonb_path_ops поддерживает поиск по вложению (@>); в MongoDB — составной индекс по полям внутри массива.
	if _, err := pg.Exec(ctx, `CREATE INDEX exp_events_gin ON `+pgTable+` USING gin (events jsonb_path_ops)`); err != nil {
		return err
	}
	if _, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "events.type", Value: 1}, {Key: "events.delay_seconds", Value: 1}}}); err != nil {
		return err
	}

	fmt.Printf("Сценарий: журнал событий рейса. %d журналов по %d событий (всего %d событий), одни и те же данные в обоих хранилищах.\n", journals, eventsPer, journals*eventsPer)
	fmt.Printf("Время — миллисекунды. Точечные операции — среднее из %d повторов, запросы по всем данным — медиана из %d запусков.\n\n", reads, queryRuns)
	var table []row

	// ---------- данные ----------
	rng := rand.New(rand.NewSource(42))
	all := make([][]event, journals)
	for i := range all {
		all[i] = genEvents(rng, eventsPer)
	}
	lastAt := all[0][eventsPer-1].At

	// ---------- загрузка ----------
	t := time.Now()
	copyRows := make([][]any, journals)
	for i, evs := range all {
		raw, _ := json.Marshal(evs)
		copyRows[i] = []any{int64(i + 1), raw, evs[len(evs)-1].At}
	}
	if _, err := pg.CopyFrom(ctx, pgx.Identifier{pgTable}, []string{"trip_id", "events", "last_event_at"}, pgx.CopyFromRows(copyRows)); err != nil {
		return fmt.Errorf("загрузка в PostgreSQL: %w", err)
	}
	pgLoad := time.Since(t)
	t = time.Now()
	for start := 0; start < journals; start += 500 {
		end := min(start+500, journals)
		docs := make([]any, 0, end-start)
		for i := start; i < end; i++ {
			docs = append(docs, bson.D{{Key: "_id", Value: int64(i + 1)}, {Key: "events", Value: all[i]}, {Key: "last_event_at", Value: all[i][eventsPer-1].At}})
		}
		if _, err := coll.InsertMany(ctx, docs); err != nil {
			return fmt.Errorf("загрузка в MongoDB: %w", err)
		}
	}
	mgLoad := time.Since(t)
	table = append(table, row{"загрузка всех данных", ms(pgLoad), ms(mgLoad)})

	// ---------- размер на диске ----------
	var pgData, pgIdx int64
	if err := pg.QueryRow(ctx, `SELECT pg_relation_size($1) + coalesce(pg_total_relation_size(reltoastrelid),0), pg_indexes_size($1) FROM pg_class WHERE oid = $1::regclass`, pgTable).Scan(&pgData, &pgIdx); err != nil {
		return err
	}
	// Статистика хранилища MongoDB обновляется при контрольной точке: принудительно сбрасываем данные на диск.
	if err := mc.Database("admin").RunCommand(ctx, bson.D{{Key: "fsync", Value: 1}}).Err(); err != nil {
		return err
	}
	var stats struct {
		Size           int64 `bson:"size"`
		StorageSize    int64 `bson:"storageSize"`
		TotalIndexSize int64 `bson:"totalIndexSize"`
	}
	if err := mdb.RunCommand(ctx, bson.D{{Key: "collStats", Value: mongoColl}}).Decode(&stats); err != nil {
		return err
	}
	var pgLogical int64
	if err := pg.QueryRow(ctx, `SELECT sum(octet_length(events::text)) FROM `+pgTable).Scan(&pgLogical); err != nil {
		return err
	}
	kb := func(b int64) string { return fmt.Sprintf("%d КБ", b/1024) }
	table = append(table, row{"размер данных до сжатия (логический)", kb(pgLogical), kb(stats.Size)})
	table = append(table, row{"данные на диске (со сжатием, без индексов)", kb(pgData), kb(stats.StorageSize)})
	table = append(table, row{"индексы на диске", kb(pgIdx), kb(stats.TotalIndexSize)})

	// ---------- точечное чтение журнала целиком ----------
	ids := make([]int64, reads)
	for i := range ids {
		ids[i] = int64(rng.Intn(journals) + 1)
	}
	t = time.Now()
	for _, id := range ids {
		var raw []byte
		if err := pg.QueryRow(ctx, `SELECT events FROM `+pgTable+` WHERE trip_id = $1`, id).Scan(&raw); err != nil {
			return err
		}
		var evs []event
		if err := json.Unmarshal(raw, &evs); err != nil {
			return err
		}
	}
	pgRead := time.Since(t) / reads
	t = time.Now()
	for _, id := range ids {
		var doc struct {
			Events []event `bson:"events"`
		}
		if err := coll.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc); err != nil {
			return err
		}
	}
	mgRead := time.Since(t) / reads
	table = append(table, row{"прочитать журнал рейса целиком", ms(pgRead), ms(mgRead)})

	// ---------- добавление события (с проверкой хронологии в самом запросе) ----------
	newAt := lastAt.Add(time.Minute)
	newEv := event{At: newAt, AtText: newAt.Format(time.RFC3339), Type: "NOTE", Note: "добавлено в эксперименте"}
	newRaw, _ := json.Marshal([]event{newEv})
	t = time.Now()
	for _, id := range ids[:100] {
		tag, err := pg.Exec(ctx, `UPDATE `+pgTable+` SET events = events || $1::jsonb, last_event_at = $2 WHERE trip_id = $3 AND last_event_at <= $2`, string(newRaw), newAt, id)
		if err != nil {
			return err
		}
		_ = tag
	}
	pgAppend := time.Since(t) / 100
	t = time.Now()
	for _, id := range ids[:100] {
		if _, err := coll.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "last_event_at", Value: bson.D{{Key: "$not", Value: bson.D{{Key: "$gt", Value: newAt}}}}}},
			bson.D{{Key: "$push", Value: bson.D{{Key: "events", Value: newEv}}}, {Key: "$set", Value: bson.D{{Key: "last_event_at", Value: newAt}}}}); err != nil {
			return err
		}
	}
	mgAppend := time.Since(t) / 100
	table = append(table, row{"добавить событие в конец (с проверкой порядка)", ms(pgAppend), ms(mgAppend)})

	// ---------- запросы по всем данным (результаты сверяем между хранилищами) ----------
	type query struct {
		name string
		pg   func() (int64, error)
		mg   func() (int64, error)
	}
	pgCount := func(sql string, args ...any) func() (int64, error) {
		return func() (int64, error) {
			var n int64
			err := pg.QueryRow(ctx, sql, args...).Scan(&n)
			return n, err
		}
	}
	mgCount := func(filter bson.D) func() (int64, error) {
		return func() (int64, error) { return coll.CountDocuments(ctx, filter) }
	}
	queries := []query{
		{"найти журналы с задержкой ровно 600 с (равенство)",
			pgCount(`SELECT count(*) FROM ` + pgTable + ` WHERE events @> '[{"type":"DELAY_DETECTED","delay_seconds":600}]'`),
			mgCount(bson.D{{Key: "events", Value: bson.D{{Key: "$elemMatch", Value: bson.D{{Key: "type", Value: "DELAY_DETECTED"}, {Key: "delay_seconds", Value: targetSecs}}}}}})},
		{"найти журналы с задержкой больше 850 с (диапазон)",
			pgCount(`SELECT count(*) FROM ` + pgTable + ` WHERE jsonb_path_exists(events, '$[*] ? (@.type == "DELAY_DETECTED" && @.delay_seconds > 850)')`),
			mgCount(bson.D{{Key: "events", Value: bson.D{{Key: "$elemMatch", Value: bson.D{{Key: "type", Value: "DELAY_DETECTED"}, {Key: "delay_seconds", Value: bson.D{{Key: "$gt", Value: rangeSecs}}}}}}}})},
	}
	for _, q := range queries {
		var pgT, mgT []time.Duration
		var pgN, mgN int64
		for r := 0; r < queryRuns; r++ {
			t = time.Now()
			if pgN, err = q.pg(); err != nil {
				return err
			}
			pgT = append(pgT, time.Since(t))
			t = time.Now()
			if mgN, err = q.mg(); err != nil {
				return err
			}
			mgT = append(mgT, time.Since(t))
		}
		if pgN != mgN {
			return fmt.Errorf("результаты разошлись в запросе %q: PostgreSQL=%d MongoDB=%d", q.name, pgN, mgN)
		}
		table = append(table, row{fmt.Sprintf("%s → %d журналов", q.name, pgN), ms(median(pgT)), ms(median(mgT))})
	}

	// аналитика: число событий каждого типа по всем журналам
	typeCountsPG := func() (map[string]int64, error) {
		rows, err := pg.Query(ctx, `SELECT e->>'type', count(*) FROM `+pgTable+`, jsonb_array_elements(events) e GROUP BY 1`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := map[string]int64{}
		for rows.Next() {
			var k string
			var n int64
			if err := rows.Scan(&k, &n); err != nil {
				return nil, err
			}
			out[k] = n
		}
		return out, rows.Err()
	}
	typeCountsMongo := func() (map[string]int64, error) {
		cur, err := coll.Aggregate(ctx, mongo.Pipeline{
			{{Key: "$unwind", Value: "$events"}},
			{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$events.type"}, {Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}}}}},
		})
		if err != nil {
			return nil, err
		}
		var rows []struct {
			ID string `bson:"_id"`
			N  int64  `bson:"n"`
		}
		if err := cur.All(ctx, &rows); err != nil {
			return nil, err
		}
		out := map[string]int64{}
		for _, r := range rows {
			out[r.ID] = r.N
		}
		return out, nil
	}
	var pgT, mgT []time.Duration
	var pgTypes, mgTypes map[string]int64
	for r := 0; r < queryRuns; r++ {
		t = time.Now()
		if pgTypes, err = typeCountsPG(); err != nil {
			return err
		}
		pgT = append(pgT, time.Since(t))
		t = time.Now()
		if mgTypes, err = typeCountsMongo(); err != nil {
			return err
		}
		mgT = append(mgT, time.Since(t))
	}
	for k, v := range pgTypes {
		if mgTypes[k] != v {
			return fmt.Errorf("аналитика разошлась по типу %s: PostgreSQL=%d MongoDB=%d", k, v, mgTypes[k])
		}
	}
	table = append(table, row{"аналитика: число событий каждого типа по всем журналам", ms(median(pgT)), ms(median(mgT))})

	fmt.Printf("%-66s %16s %16s\n", "операция", "PostgreSQL jsonb", "MongoDB")
	for _, r := range table {
		fmt.Printf("%-66s %16s %16s\n", r.name, r.pg, r.mg)
	}
	fmt.Println("\nРезультаты запросов и аналитики в обоих хранилищах совпали (проверка выполнялась автоматически).")

	// ---------- защита данных ----------
	fmt.Println("\nЗащита данных: попытка записать некорректное событие (DELAY_DETECTED без delay_seconds)")
	bad := `[{"at":"2026-10-01T09:00:00Z","type":"DELAY_DETECTED"}]`
	if _, err := pg.Exec(ctx, `INSERT INTO `+pgTable+` VALUES (999999, $1::jsonb, now())`, bad); err != nil {
		fmt.Println("  PostgreSQL jsonb без ограничений: ОТКЛОНЕНО (неожиданно):", err)
	} else {
		fmt.Println("  PostgreSQL jsonb без ограничений:  ПРИНЯТО (структуру внутри jsonb база сама не проверяет)")
		pg.Exec(ctx, `DELETE FROM `+pgTable+` WHERE trip_id = 999999`)
	}
	if _, err := pg.Exec(ctx, `ALTER TABLE `+pgTable+` ADD CONSTRAINT exp_delay_required CHECK (NOT jsonb_path_exists(events, '$[*] ? (@.type == "DELAY_DETECTED" && !exists(@.delay_seconds))'))`); err != nil {
		return err
	}
	var pgErr *pgconn.PgError
	if _, err := pg.Exec(ctx, `INSERT INTO `+pgTable+` VALUES (999999, $1::jsonb, now())`, bad); errors.As(err, &pgErr) {
		fmt.Printf("  PostgreSQL jsonb + CHECK:           ОТКЛОНЕНО (код %s, ограничение %s) — правило пришлось записать вручную на языке jsonpath\n", pgErr.Code, pgErr.ConstraintName)
	} else {
		fmt.Println("  PostgreSQL jsonb + CHECK:           принято (неожиданно)")
	}
	// в MongoDB — настоящая схема нашего проекта (валидатор $jsonSchema)
	if err := mongorepo.EnsureSchema(ctx, mdb); err != nil {
		return err
	}
	_, err = mdb.Collection(mongorepo.CollJournals).InsertOne(ctx, bson.M{"_id": int64(1), "events": bson.A{bson.M{"at": base, "type": "DELAY_DETECTED"}},
		"last_event_at": base, "created_at": base, "updated_at": base, "schema_version": int32(1)})
	var we mongo.WriteException
	if errors.As(err, &we) && len(we.WriteErrors) > 0 && we.WriteErrors[0].Code == 121 {
		fmt.Println("  MongoDB со схемой валидации:        ОТКЛОНЕНО (код 121, DocumentValidationFailure) — схема описана декларативно, в том числе типы и лишние поля")
	} else {
		fmt.Println("  MongoDB со схемой валидации:        принято (неожиданно):", err)
	}
	_, err = mdb.Collection(mongorepo.CollJournals).InsertOne(ctx, bson.M{"_id": int64(2), "events": bson.A{bson.M{"at": "вчера", "type": "DEPARTED"}},
		"last_event_at": base, "created_at": base, "updated_at": base, "schema_version": int32(1)})
	if errors.As(err, &we) {
		fmt.Println("  Время строкой вместо даты:          MongoDB ОТКЛОНЯЕТ (тип date проверяется); в jsonb время — всегда строка, типа «дата» там нет")
	}
	return nil
}
