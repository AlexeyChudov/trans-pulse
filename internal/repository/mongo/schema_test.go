package mongorepo_test

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Проверяем сами правила схемы валидации (последний рубеж защиты данных): допустимый документ проходит,
// каждый недопустимый отклоняется базой с ошибкой DocumentValidationFailure.

func goodPoint() bson.M {
	return bson.M{"ts": base, "lat": 50.45, "lon": 30.52, "speed": 40.0}
}

func goodBucket() bson.M {
	return bson.M{"vehicle_id": int64(1), "hour": base, "count": int32(1), "points": bson.A{goodPoint()}, "schema_version": int32(1)}
}

func with(doc bson.M, key string, value any) bson.M {
	out := bson.M{}
	for k, v := range doc {
		out[k] = v
	}
	if value == nil {
		delete(out, key)
	} else {
		out[key] = value
	}
	return out
}

func TestSchemaTelemetryBuckets(t *testing.T) {
	db := testDB(t)
	if err := insertRaw(db, "telemetry_buckets", goodBucket()); err != nil {
		t.Fatalf("корректная корзина должна проходить: %v", err)
	}
	bad := map[string]bson.M{
		"нет vehicle_id":           with(goodBucket(), "vehicle_id", nil),
		"vehicle_id не число":      with(goodBucket(), "vehicle_id", "один"),
		"нет времени измерения":    with(goodBucket(), "points", bson.A{bson.M{"lat": 50.0, "lon": 30.0, "speed": 1.0}}),
		"широта больше 90":         with(goodBucket(), "points", bson.A{with(goodPoint(), "lat", 91.0)}),
		"долгота меньше -180":      with(goodBucket(), "points", bson.A{with(goodPoint(), "lon", -181.0)}),
		"отрицательная скорость":   with(goodBucket(), "points", bson.A{with(goodPoint(), "speed", -1.0)}),
		"время строкой":            with(goodBucket(), "points", bson.A{with(goodPoint(), "ts", "вчера")}),
		"лишнее поле у точки":      with(goodBucket(), "points", bson.A{with(goodPoint(), "color", "red")}),
		"неизвестная версия схемы": with(goodBucket(), "schema_version", int32(99)),
	}
	for name, doc := range bad {
		doc = with(doc, "hour", base.Add(time.Duration(len(name))*time.Hour)) // разные ключи, чтобы не мешал уникальный индекс
		if err := insertRaw(db, "telemetry_buckets", doc); !isValidationFailure(err) {
			t.Errorf("%s: ожидалась ошибка валидации, получено %v", name, err)
		}
	}
}

func goodEvent() bson.M { return bson.M{"at": base, "type": "DEPARTED"} }

func goodJournal() bson.M {
	return bson.M{"_id": int64(1), "events": bson.A{}, "last_event_at": nil, "created_at": base, "updated_at": base, "schema_version": int32(1)}
}

func TestSchemaTripJournals(t *testing.T) {
	db := testDB(t)
	if err := insertRaw(db, "trip_journals", with(goodJournal(), "events", bson.A{goodEvent(), bson.M{"at": base, "type": "DELAY_DETECTED", "delay_seconds": int32(420)}, bson.M{"at": base, "type": "ARRIVED_AT_STOP", "stop_id": int64(5)}})); err != nil {
		t.Fatalf("корректный журнал должен проходить: %v", err)
	}
	bad := map[string]bson.A{
		"неизвестный тип события":          {bson.M{"at": base, "type": "TELEPORTED"}},
		"DELAY_DETECTED без delay_seconds": {bson.M{"at": base, "type": "DELAY_DETECTED"}},
		"ARRIVED_AT_STOP без stop_id":      {bson.M{"at": base, "type": "ARRIVED_AT_STOP"}},
		"нет времени события":              {bson.M{"type": "DEPARTED"}},
		"отрицательная задержка":           {bson.M{"at": base, "type": "DELAY_DETECTED", "delay_seconds": int32(-5)}},
		"слишком длинная заметка":          {bson.M{"at": base, "type": "NOTE", "note": string(make([]rune, 501))}},
	}
	id := int64(100)
	for name, events := range bad {
		id++
		doc := with(with(goodJournal(), "events", events), "_id", id)
		if err := insertRaw(db, "trip_journals", doc); !isValidationFailure(err) {
			t.Errorf("%s: ожидалась ошибка валидации, получено %v", name, err)
		}
	}
}

func goodCase() bson.M {
	return bson.M{"_id": int64(1), "trip_id": int64(2), "vehicle_id": int64(3), "route_id": int64(4), "title": "Задержка рейса",
		"tags": bson.A{"delay"}, "status": "OPEN", "comments": bson.A{}, "created_by": int64(7), "created_at": base, "updated_at": base, "schema_version": int32(1)}
}

func TestSchemaIncidentCases(t *testing.T) {
	db := testDB(t)
	if err := insertRaw(db, "incident_cases", goodCase()); err != nil {
		t.Fatalf("корректная карточка должна проходить: %v", err)
	}
	closed := with(with(goodCase(), "status", "CLOSED"), "closed_at", base)
	bad := map[string]bson.M{
		"CLOSED без closed_by":    closed,
		"CLOSED без closed_at":    with(with(goodCase(), "status", "CLOSED"), "closed_by", int64(7)),
		"OPEN, но есть closed_at": with(goodCase(), "closed_at", base),
		"неизвестный статус":      with(goodCase(), "status", "ARCHIVED"),
		"короткий заголовок":      with(goodCase(), "title", "ab"),
		"дубликаты тегов":         with(goodCase(), "tags", bson.A{"a", "a"}),
		"больше 10 тегов":         with(goodCase(), "tags", bson.A{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}),
		"комментарий без автора":  with(goodCase(), "comments", bson.A{bson.M{"text": "x", "at": base}}),
		"пустой комментарий":      with(goodCase(), "comments", bson.A{bson.M{"author_id": int64(1), "text": "", "at": base}}),
	}
	id := int64(100)
	for name, doc := range bad {
		id++
		if err := insertRaw(db, "incident_cases", with(doc, "_id", id)); !isValidationFailure(err) {
			t.Errorf("%s: ожидалась ошибка валидации, получено %v", name, err)
		}
	}
	valid := with(with(with(goodCase(), "_id", int64(50)), "status", "CLOSED"), "closed_at", base)
	valid = with(valid, "closed_by", int64(7))
	if err := insertRaw(db, "incident_cases", valid); err != nil {
		t.Errorf("корректно закрытая карточка должна проходить: %v", err)
	}
}

func goodStop() bson.M {
	return bson.M{"_id": int64(1), "name": "Вокзал", "location": bson.M{"type": "Point", "coordinates": bson.A{30.5118, 50.4419}},
		"route_ids": bson.A{}, "updated_at": base, "schema_version": int32(1)}
}

func TestSchemaStopPointsAndRouteProfiles(t *testing.T) {
	db := testDB(t)
	if err := insertRaw(db, "stop_points", goodStop()); err != nil {
		t.Fatalf("корректная остановка должна проходить: %v", err)
	}
	bad := map[string]bson.M{
		"тип геометрии не Point":     with(goodStop(), "location", bson.M{"type": "Polygon", "coordinates": bson.A{30.5, 50.4}}),
		"широта вне диапазона":       with(goodStop(), "location", bson.M{"type": "Point", "coordinates": bson.A{30.5, 95.0}}),
		"долгота вне диапазона":      with(goodStop(), "location", bson.M{"type": "Point", "coordinates": bson.A{190.0, 50.4}}),
		"одна координата":            with(goodStop(), "location", bson.M{"type": "Point", "coordinates": bson.A{30.5}}),
		"дубликаты route_ids":        with(goodStop(), "route_ids", bson.A{int64(1), int64(1)}),
		"атрибут недопустимого типа": with(goodStop(), "attributes", bson.M{"shelter": bson.A{1, 2}}),
	}
	id := int64(100)
	for name, doc := range bad {
		id++
		if err := insertRaw(db, "stop_points", with(doc, "_id", id)); !isValidationFailure(err) {
			t.Errorf("%s: ожидалась ошибка валидации, получено %v", name, err)
		}
	}

	profile := bson.M{"_id": int64(1), "stops": bson.A{bson.M{"stop_id": int64(1), "seq": int32(1)}}, "updated_at": base, "schema_version": int32(1)}
	if err := insertRaw(db, "route_profiles", profile); err != nil {
		t.Fatalf("корректный профиль должен проходить: %v", err)
	}
	badProfile := with(with(profile, "_id", int64(2)), "stops", bson.A{bson.M{"stop_id": int64(1), "seq": int32(0)}})
	if err := insertRaw(db, "route_profiles", badProfile); !isValidationFailure(err) {
		t.Errorf("seq=0: ожидалась ошибка валидации, получено %v", err)
	}
}
