package domain

import "time"

const MaxCaseComments = 200 // граница встроенного массива комментариев

type CaseStatus string

const (
	CaseOpen   CaseStatus = "OPEN"
	CaseClosed CaseStatus = "CLOSED"
)

type CaseComment struct {
	AuthorID int64 // ссылка на пользователя в PostgreSQL
	Text     string
	At       time.Time
}

// IncidentCase — карточка расследования инцидента. Рейс, автобус, маршрут и авторы — ссылки на идентификаторы PostgreSQL,
// комментарии встроены в документ.
type IncidentCase struct {
	IncidentID int64
	TripID     int64
	VehicleID  int64
	RouteID    int64
	Title      string
	Tags       []string
	Status     CaseStatus
	Comments   []CaseComment
	CreatedBy  int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ClosedAt   *time.Time
	ClosedBy   *int64
}
