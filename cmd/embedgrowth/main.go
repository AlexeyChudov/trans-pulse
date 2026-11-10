// Команда embedgrowth — эксперимент: как растут размер и время операций с документом, когда встроенный массив событий
// становится всё длиннее, и где проходит жёсткая граница (16 МиБ на документ в MongoDB).
// Запуск: set -a; . ./.env; set +a; go run ./cmd/embedgrowth
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	dbName        = "trans_pulse_experiment"
	serverMaxBSON = 16 * 1024 * 1024 // жёсткий предел размера вставляемого документа: 16 МиБ (внутренний запас сервера — 16793600 байт — для приложения недоступен)
	repeats       = 5
	appBoundary   = 500 // граница, выбранная в приложении (domain.MaxJournalEvents)
	softBoundary  = 1 << 20
)

type event struct {
	At     time.Time `bson:"at"`
	Type   string    `bson:"type"`
	StopID int64     `bson:"stop_id"`
	Note   string    `bson:"note"`
}

var base = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

func newEvent(i int) event {
	return event{At: base.Add(time.Duration(i) * time.Second), Type: "ARRIVED_AT_STOP", StopID: int64(i % 200), Note: "прибытие на остановку"}
}

func buildDoc(id, n int) bson.D {
	events := make([]event, n)
	for i := range events {
		events[i] = newEvent(i)
	}
	return bson.D{{Key: "_id", Value: id}, {Key: "events", Value: events}}
}

func median(d []time.Duration) time.Duration {
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	return d[len(d)/2]
}

func ms(d time.Duration) string { return fmt.Sprintf("%.2f", float64(d.Microseconds())/1000) }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		return fmt.Errorf("задайте MONGO_URI")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)
	db := client.Database(dbName)
	defer db.Drop(ctx)
	coll := db.Collection("growth")
	if err := db.Drop(ctx); err != nil {
		return err
	}

	fmt.Println("Эксперимент: рост встроенного массива events (событие: время, тип, остановка, заметка)")
	fmt.Printf("Каждое измерение — медиана из %d запусков, время в миллисекундах.\n\n", repeats)
	fmt.Printf("%9s %12s %11s %14s %17s %20s\n", "событий", "размер", "вставка", "чтение целиком", "добавить событие", "прочитать последнее")

	for i, n := range []int{100, 500, 1000, 5000, 10000, 50000, 100000} {
		doc := buildDoc(i+1, n)
		raw, err := bson.Marshal(doc)
		if err != nil {
			return err
		}
		t := time.Now()
		if _, err := coll.InsertOne(ctx, doc); err != nil {
			return fmt.Errorf("вставка %d событий: %w", n, err)
		}
		insert := time.Since(t)

		var readAll, push, readLast []time.Duration
		for r := 0; r < repeats; r++ {
			t = time.Now()
			var out struct {
				Events []event `bson:"events"`
			}
			if err := coll.FindOne(ctx, bson.D{{Key: "_id", Value: i + 1}}).Decode(&out); err != nil {
				return err
			}
			readAll = append(readAll, time.Since(t))

			t = time.Now()
			if _, err := coll.UpdateOne(ctx, bson.D{{Key: "_id", Value: i + 1}}, bson.D{{Key: "$push", Value: bson.D{{Key: "events", Value: newEvent(n + r)}}}}); err != nil {
				return err
			}
			push = append(push, time.Since(t))

			t = time.Now()
			var last struct {
				Events []event `bson:"events"`
			}
			err := coll.FindOne(ctx, bson.D{{Key: "_id", Value: i + 1}},
				options.FindOne().SetProjection(bson.D{{Key: "events", Value: bson.D{{Key: "$slice", Value: -1}}}})).Decode(&last)
			if err != nil {
				return err
			}
			readLast = append(readLast, time.Since(t))
		}
		fmt.Printf("%9d %9.0f КБ %11s %14s %17s %20s\n", n, float64(len(raw))/1024, ms(insert), ms(median(readAll)), ms(median(push)), ms(median(readLast)))
	}

	hard := maxFit(serverMaxBSON)
	soft := maxFit(softBoundary)
	fmt.Printf("\nЖёсткая граница MongoDB: %d байт (16 МиБ) на документ. Точная граница для нашего события — %d событий.\n", serverMaxBSON, hard)
	for _, c := range []struct {
		label string
		n     int
		id    int
	}{{"на границе", hard, 100}, {"на 1 событие больше", hard + 1, 101}} {
		doc := buildDoc(c.id, c.n)
		_, err := coll.InsertOne(ctx, doc)
		status := "вставлен"
		if err != nil {
			status = "ОТКЛОНЁН сервером: " + firstLine(err.Error())
		}
		fmt.Printf("  %-20s %7d событий, %8d байт → %s\n", c.label, c.n, sizeOf(c.n), status)
	}

	fmt.Printf("\nВыводы для проекта:\n")
	fmt.Printf("  • до ~%d событий (~1 МиБ) документ читается и обновляется быстро; дальше целиком его читать уже дорого;\n", soft)
	fmt.Printf("  • жёсткий предел — %d событий; упереться в него нельзя: запись отклоняется целиком;\n", hard)
	fmt.Printf("  • журнал рейса ограничен %d событиями (%d КБ): это в %d раз меньше мягкой границы 1 МиБ и в %d раз меньше жёсткого предела;\n",
		appBoundary, sizeOf(appBoundary)/1024, soft/appBoundary, hard/appBoundary)
	fmt.Printf("  • добавление события ($push) дорожает с длиной массива медленнее, чем чтение целиком, а прочитать последнее событие дёшево (проекция $slice).\n")
	return nil
}

// sizeOf — размер BSON-документа с n событиями (индексы элементов массива удлиняются с ростом n, поэтому считаем точно).
func sizeOf(n int) int {
	raw, _ := bson.Marshal(buildDoc(100, n))
	return len(raw)
}

// maxFit — наибольшее число событий, при котором документ не превышает limit байт (бисекция).
func maxFit(limit int) int {
	lo, hi := 1, 400000
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if sizeOf(mid) <= limit {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	if len(s) > 140 {
		return s[:140] + "…"
	}
	return s
}
