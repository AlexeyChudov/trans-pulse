package domain

import "time"

const MaxRouteStops = 200

// StopPoint — документная проекция остановки: имя и координаты копируются из PostgreSQL (источник истины),
// route_ids — обратные ссылки для связи M:N, attributes — гибкие данные, которыми владеет MongoDB.
type StopPoint struct {
	StopID     int64
	Name       string
	Latitude   float64
	Longitude  float64
	RouteIDs   []int64
	Attributes map[string]any
	UpdatedAt  time.Time
}

// NearStop — результат геопоиска: остановка и расстояние до точки поиска.
type NearStop struct {
	StopPoint
	DistanceM float64
}

type RouteStopRef struct {
	StopID int64
	Seq    int
}

// RouteProfile — состав маршрута: упорядоченные ссылки на остановки (связь M:N между маршрутами и остановками).
type RouteProfile struct {
	RouteID   int64
	Stops     []RouteStopRef
	UpdatedAt time.Time
}
