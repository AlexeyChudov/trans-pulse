package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

const (
	maxAttributes = 20
	maxRadiusM    = 5000
	defaultNear   = 10
	maxNear       = 50
)

// GeoService управляет документной моделью маршрутов и остановок. PostgreSQL остаётся источником истины
// для маршрутов и остановок; в MongoDB лежат проекция остановок (с геопоиском и гибкими атрибутами) и состав маршрута.
// Связь M:N хранится ссылками с двух сторон: у маршрута — упорядоченные stop_id, у остановки — route_ids.
type GeoService struct {
	stopPoints domain.StopPointRepository
	profiles   domain.RouteProfileRepository
	routes     domain.RouteRepository // PostgreSQL
	now        func() time.Time
}

func NewGeoService(stopPoints domain.StopPointRepository, profiles domain.RouteProfileRepository, routes domain.RouteRepository) *GeoService {
	return &GeoService{stopPoints: stopPoints, profiles: profiles, routes: routes, now: time.Now}
}

// CreateStopPoint создаёт проекцию остановки: имя и координаты копируются из PostgreSQL.
func (s *GeoService) CreateStopPoint(ctx context.Context, stopID int64, attrs map[string]any) (domain.StopPoint, error) {
	if err := validateAttributes(attrs); err != nil {
		return domain.StopPoint{}, err
	}
	stop, err := s.routes.GetStop(ctx, stopID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.StopPoint{}, invalid("остановка не найдена в PostgreSQL")
	}
	if err != nil {
		return domain.StopPoint{}, err
	}
	sp := domain.StopPoint{StopID: stop.ID, Name: stop.Name, Latitude: stop.Latitude, Longitude: stop.Longitude, Attributes: attrs}
	if err := s.stopPoints.Create(ctx, &sp); err != nil {
		return domain.StopPoint{}, err
	}
	return sp, nil
}

func (s *GeoService) GetStopPoint(ctx context.Context, stopID int64) (domain.StopPoint, error) {
	return s.stopPoints.Get(ctx, stopID)
}

// SetAttributes заменяет гибкие атрибуты остановки (укрытие, скамейки и т. п.) — данные, которыми владеет MongoDB.
func (s *GeoService) SetAttributes(ctx context.Context, stopID int64, attrs map[string]any) (domain.StopPoint, error) {
	if err := validateAttributes(attrs); err != nil {
		return domain.StopPoint{}, err
	}
	return s.stopPoints.SetAttributes(ctx, stopID, attrs, s.now())
}

// DeleteStopPoint: пока остановка входит в маршруты, удалить проекцию нельзя (ErrConflict от репозитория).
func (s *GeoService) DeleteStopPoint(ctx context.Context, stopID int64) error {
	return s.stopPoints.Delete(ctx, stopID)
}

func (s *GeoService) Near(ctx context.Context, lat, lon float64, radiusM, limit int) ([]domain.NearStop, error) {
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return nil, invalid("координаты вне допустимых пределов")
	}
	if radiusM < 1 || radiusM > maxRadiusM {
		return nil, invalid("радиус: от 1 до %d метров", maxRadiusM)
	}
	if limit <= 0 {
		limit = defaultNear
	}
	if limit > maxNear {
		limit = maxNear
	}
	return s.stopPoints.Near(ctx, lat, lon, radiusM, limit)
}

// SetRouteStops задаёт состав маршрута. Правила: маршрут есть в PostgreSQL; остановок от 1 до 200;
// нумерация строго 1..N без пропусков и повторов; остановка не повторяется; у каждой остановки есть проекция в MongoDB.
// Затем обновляются обратные ссылки route_ids. Транзакций между документами нет, поэтому порядок шагов выбран безопасным:
// сначала добавляем ссылки, потом заменяем профиль, в конце убираем ссылки у выбывших остановок.
// Лишняя обратная ссылка безвредна, отсутствующая — нет, поэтому сбой посередине не ломает данные.
func (s *GeoService) SetRouteStops(ctx context.Context, routeID int64, refs []domain.RouteStopRef) (domain.RouteProfile, error) {
	if _, err := s.routes.Get(ctx, routeID); err != nil {
		return domain.RouteProfile{}, err
	}
	sorted, err := validateRouteStops(refs)
	if err != nil {
		return domain.RouteProfile{}, err
	}
	for _, r := range sorted {
		if _, err := s.stopPoints.Get(ctx, r.StopID); errors.Is(err, domain.ErrNotFound) {
			return domain.RouteProfile{}, invalid("остановка %d не найдена в MongoDB: сначала создайте её проекцию", r.StopID)
		} else if err != nil {
			return domain.RouteProfile{}, err
		}
	}
	for _, r := range sorted {
		if err := s.stopPoints.AddRoute(ctx, r.StopID, routeID); err != nil {
			return domain.RouteProfile{}, err
		}
	}
	profile := domain.RouteProfile{RouteID: routeID, Stops: sorted}
	prev, err := s.profiles.Replace(ctx, &profile)
	if err != nil {
		return domain.RouteProfile{}, err
	}
	if prev != nil {
		keep := map[int64]bool{}
		for _, r := range sorted {
			keep[r.StopID] = true
		}
		for _, r := range prev.Stops {
			if !keep[r.StopID] {
				if err := s.stopPoints.RemoveRoute(ctx, r.StopID, routeID); err != nil && !errors.Is(err, domain.ErrNotFound) {
					return domain.RouteProfile{}, err
				}
			}
		}
	}
	return profile, nil
}

func (s *GeoService) GetRouteProfile(ctx context.Context, routeID int64) (domain.RouteProfile, error) {
	return s.profiles.Get(ctx, routeID)
}

func (s *GeoService) DeleteRouteProfile(ctx context.Context, routeID int64) error {
	deleted, err := s.profiles.Delete(ctx, routeID)
	if err != nil {
		return err
	}
	for _, r := range deleted.Stops {
		if err := s.stopPoints.RemoveRoute(ctx, r.StopID, routeID); err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
	}
	return nil
}

func validateRouteStops(refs []domain.RouteStopRef) ([]domain.RouteStopRef, error) {
	if len(refs) < 1 || len(refs) > domain.MaxRouteStops {
		return nil, invalid("в маршруте должно быть от 1 до %d остановок", domain.MaxRouteStops)
	}
	sorted := append([]domain.RouteStopRef{}, refs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Seq < sorted[j].Seq })
	seen := map[int64]bool{}
	for i, r := range sorted {
		if r.Seq != i+1 {
			return nil, invalid("нумерация остановок должна идти строго 1..%d без пропусков и повторов", len(sorted))
		}
		if r.StopID <= 0 {
			return nil, invalid("некорректный номер остановки")
		}
		if seen[r.StopID] {
			return nil, invalid("остановка %d указана в маршруте дважды", r.StopID)
		}
		seen[r.StopID] = true
	}
	return sorted, nil
}

func validateAttributes(attrs map[string]any) error {
	if len(attrs) > maxAttributes {
		return invalid("не более %d атрибутов", maxAttributes)
	}
	for k, v := range attrs {
		if n := utf8.RuneCountInString(strings.TrimSpace(k)); n < 1 || n > 50 {
			return invalid("имя атрибута: 1–50 символов")
		}
		switch x := v.(type) {
		case string:
			if utf8.RuneCountInString(x) > 200 {
				return invalid("значение атрибута %q длиннее 200 символов", k)
			}
		case bool, int, int32, int64, float32, float64:
		default:
			return invalid("значение атрибута %q: допустимы строка, число или логическое значение", k)
		}
	}
	return nil
}
