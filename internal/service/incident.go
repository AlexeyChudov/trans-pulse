package service

import (
	"context"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

type IncidentService struct {
	repo domain.IncidentRepository
	now  func() time.Time
}

func NewIncidentService(repo domain.IncidentRepository, now func() time.Time) *IncidentService {
	return &IncidentService{repo: repo, now: clockOrNow(now)}
}

func (s *IncidentService) List(ctx context.Context, f domain.IncidentFilter) ([]domain.Incident, error) {
	if f.Status != "" && !f.Status.IsValid() {
		return nil, invalid("неизвестный статус инцидента")
	}
	f.Limit, f.Offset = NormalizePage(f.Limit, f.Offset)
	return s.repo.List(ctx, f)
}

func (s *IncidentService) Get(ctx context.Context, id int64) (domain.Incident, error) {
	return s.repo.Get(ctx, id)
}

// Resolve закрывает инцидент от имени пользователя (userID берётся из токена).
func (s *IncidentService) Resolve(ctx context.Context, id, userID int64) (domain.Incident, error) {
	return s.repo.Resolve(ctx, id, userID, s.now())
}
