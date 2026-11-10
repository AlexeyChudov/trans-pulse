package domain

import "time"

type Route struct {
	ID        int64
	Number    string
	Name      string
	IsActive  bool
	CreatedAt time.Time
}

type Stop struct {
	ID        int64
	Name      string
	Latitude  float64
	Longitude float64
	IsActive  bool
	CreatedAt time.Time
}

// RouteStop — остановка в составе маршрута с порядковым номером.
type RouteStop struct {
	Stop
	SequenceNo int
}
