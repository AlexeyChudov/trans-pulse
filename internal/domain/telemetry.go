package domain

import "time"

// MaxBucketPoints — граница встроенного массива точек в почасовой корзине (при шаге 5–10 с в час набирается 360–720 точек).
const MaxBucketPoints = 720

// TelemetryPoint — одно GPS-измерение автобуса. Принадлежит MongoDB (в PostgreSQL телеметрии нет).
type TelemetryPoint struct {
	VehicleID int64
	TripID    int64 // 0 — автобус вне рейса
	Timestamp time.Time
	Latitude  float64
	Longitude float64
	SpeedKmh  float64
}
