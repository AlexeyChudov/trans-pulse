package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

type RouteService struct{ repo domain.RouteRepository }

func NewRouteService(repo domain.RouteRepository) *RouteService { return &RouteService{repo: repo} }

// List возвращает только активные маршруты.
func (s *RouteService) List(ctx context.Context) ([]domain.Route, error) {
	return s.repo.List(ctx, true)
}

func (s *RouteService) Get(ctx context.Context, id int64) (domain.Route, error) {
	return s.repo.Get(ctx, id)
}

func (s *RouteService) Create(ctx context.Context, number, name string) (domain.Route, error) {
	number, name = strings.TrimSpace(number), strings.TrimSpace(name)
	if err := validateRoute(number, name); err != nil {
		return domain.Route{}, err
	}
	r := domain.Route{Number: number, Name: name, IsActive: true}
	if err := s.repo.Create(ctx, &r); err != nil {
		return domain.Route{}, err
	}
	return r, nil
}

func (s *RouteService) Update(ctx context.Context, id int64, number, name string, isActive bool) (domain.Route, error) {
	number, name = strings.TrimSpace(number), strings.TrimSpace(name)
	if err := validateRoute(number, name); err != nil {
		return domain.Route{}, err
	}
	r, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Route{}, err
	}
	r.Number, r.Name, r.IsActive = number, name, isActive
	if err := s.repo.Update(ctx, &r); err != nil {
		return domain.Route{}, err
	}
	return r, nil
}

func (s *RouteService) Delete(ctx context.Context, id int64) error { return s.repo.Delete(ctx, id) }

// Stops возвращает остановки маршрута по порядку; ErrNotFound, если маршрута нет.
func (s *RouteService) Stops(ctx context.Context, routeID int64) ([]domain.RouteStop, error) {
	if _, err := s.repo.Get(ctx, routeID); err != nil {
		return nil, err
	}
	return s.repo.ListStops(ctx, routeID)
}

func (s *RouteService) AllStops(ctx context.Context) ([]domain.Stop, error) {
	return s.repo.ListAllStops(ctx)
}

func validateRoute(number, name string) error {
	if n := utf8.RuneCountInString(number); n < 1 || n > 20 {
		return invalid("номер маршрута: 1–20 символов")
	}
	if n := utf8.RuneCountInString(name); n < 1 || n > 100 {
		return invalid("название маршрута: 1–100 символов")
	}
	return nil
}
