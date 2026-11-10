package domain

import (
	"context"
	"time"
)

// Интерфейсы репозиториев документов (MongoDB). Реализация — internal/repository/mongo.
// Правила, которые должны выполняться атомарно (хронологический порядок, лимиты массивов, статус карточки),
// проверяются внутри самой операции записи.

type TelemetryRepository interface {
	// Append добавляет точку в почасовую корзину автобуса. ErrConflict — корзина заполнена.
	Append(ctx context.Context, p TelemetryPoint) error
	// History возвращает точки автобуса за период [from, to) по времени, не более limit штук.
	History(ctx context.Context, vehicleID int64, from, to time.Time, limit int) ([]TelemetryPoint, error)
}

type JournalRepository interface {
	Create(ctx context.Context, j *TripJournal) error // ErrConflict, если журнал рейса уже есть
	Get(ctx context.Context, tripID int64) (TripJournal, error)
	// AppendEvent добавляет событие. ErrNotFound — журнала нет; ErrConflict — событие раньше последнего
	// или журнал заполнен (MaxJournalEvents).
	AppendEvent(ctx context.Context, tripID int64, e JournalEvent) error
	Delete(ctx context.Context, tripID int64) error
}

type CaseRepository interface {
	Create(ctx context.Context, c *IncidentCase) error // ErrConflict, если карточка инцидента уже есть
	Get(ctx context.Context, incidentID int64) (IncidentCase, error)
	// Update меняет заголовок и теги (nil — не менять). Закрытую карточку менять нельзя (ErrConflict).
	Update(ctx context.Context, incidentID int64, title *string, tags []string, at time.Time) (IncidentCase, error)
	// AddComment: ErrConflict, если карточка закрыта или комментариев уже MaxCaseComments.
	AddComment(ctx context.Context, incidentID int64, c CaseComment) (IncidentCase, error)
	Close(ctx context.Context, incidentID, userID int64, at time.Time) (IncidentCase, error) // ErrConflict, если уже закрыта
	Delete(ctx context.Context, incidentID int64) error
}

type StopPointRepository interface {
	Create(ctx context.Context, s *StopPoint) error // ErrConflict, если проекция остановки уже есть
	Get(ctx context.Context, stopID int64) (StopPoint, error)
	SetAttributes(ctx context.Context, stopID int64, attrs map[string]any, at time.Time) (StopPoint, error)
	Delete(ctx context.Context, stopID int64) error // ErrConflict, пока остановка входит в какие-либо маршруты
	// Near возвращает остановки в радиусе radiusM метров, ближайшие первыми.
	Near(ctx context.Context, lat, lon float64, radiusM int, limit int) ([]NearStop, error)
	// AddRoute и RemoveRoute поддерживают обратную ссылку route_ids (идемпотентны).
	AddRoute(ctx context.Context, stopID, routeID int64) error
	RemoveRoute(ctx context.Context, stopID, routeID int64) error
}

type RouteProfileRepository interface {
	Get(ctx context.Context, routeID int64) (RouteProfile, error)
	// Replace заменяет состав маршрута целиком и возвращает предыдущий (nil, если его не было).
	Replace(ctx context.Context, p *RouteProfile) (*RouteProfile, error)
	Delete(ctx context.Context, routeID int64) (RouteProfile, error)
}
