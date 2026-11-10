package domain

import "time"

const MaxJournalEvents = 500 // граница встроенного массива событий в журнале рейса

type JournalEventType string

const (
	EventDeparted          JournalEventType = "DEPARTED"
	EventArrivedAtStop     JournalEventType = "ARRIVED_AT_STOP"
	EventDelayDetected     JournalEventType = "DELAY_DETECTED"
	EventTelemetryLost     JournalEventType = "TELEMETRY_LOST"
	EventTelemetryRestored JournalEventType = "TELEMETRY_RESTORED"
	EventNote              JournalEventType = "NOTE"
)

func (t JournalEventType) IsValid() bool {
	switch t {
	case EventDeparted, EventArrivedAtStop, EventDelayDetected, EventTelemetryLost, EventTelemetryRestored, EventNote:
		return true
	}
	return false
}

// JournalEvent — событие рейса. Поля зависят от типа (полиморфный документ): остановка нужна для ARRIVED_AT_STOP,
// задержка — для DELAY_DETECTED.
type JournalEvent struct {
	At           time.Time
	Type         JournalEventType
	StopID       int64 // 0 — не задана
	DelaySeconds int   // только для DELAY_DETECTED
	Note         string
}

// TripJournal — журнал событий рейса. События встроены в документ (читаются вместе с журналом, добавляются в конец).
type TripJournal struct {
	TripID    int64
	Events    []JournalEvent
	CreatedAt time.Time
	UpdatedAt time.Time
}
