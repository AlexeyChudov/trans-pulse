// Package service содержит бизнес-логику. Зависит только от domain: репозитории и выпуск токенов приходят через интерфейсы.
package service

import (
	"fmt"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

const (
	defaultLimit = 50
	maxLimit     = 200
)

// NormalizePage подставляет значения по умолчанию и ограничивает размер страницы.
func NormalizePage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalidInput, fmt.Sprintf(format, args...))
}

func conflict(format string, args ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrConflict, fmt.Sprintf(format, args...))
}

func clockOrNow(c func() time.Time) func() time.Time {
	if c == nil {
		return time.Now
	}
	return c
}
