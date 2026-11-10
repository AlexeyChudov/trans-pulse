package postgres_test

import (
	"errors"
	"testing"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/repository/postgres"
)

func TestUserCreateThenGet(t *testing.T) {
	repo := postgres.NewUserRepository(testPool(t))
	u := domain.User{Login: "ivan", PasswordHash: "hash", Role: domain.RoleDispatcher, IsActive: true}
	if err := repo.Create(ctx, &u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == 0 || u.CreatedAt.IsZero() {
		t.Fatalf("ID и CreatedAt должны заполняться: %+v", u)
	}

	byLogin, err := repo.GetByLogin(ctx, "ivan")
	if err != nil || byLogin.ID != u.ID || byLogin.Role != domain.RoleDispatcher || byLogin.PasswordHash != "hash" || !byLogin.IsActive {
		t.Fatalf("GetByLogin: %+v, %v", byLogin, err)
	}
	byID, err := repo.GetByID(ctx, u.ID)
	if err != nil || byID.Login != "ivan" {
		t.Fatalf("GetByID: %+v, %v", byID, err)
	}
}

func TestUserDuplicateLoginIsConflict(t *testing.T) {
	repo := postgres.NewUserRepository(testPool(t))
	repo.Create(ctx, &domain.User{Login: "ivan", PasswordHash: "h", Role: domain.RoleAdmin, IsActive: true})
	err := repo.Create(ctx, &domain.User{Login: "ivan", PasswordHash: "h", Role: domain.RoleAdmin, IsActive: true})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ожидалось ErrConflict, получено %v", err)
	}
}

func TestUserNotFound(t *testing.T) {
	repo := postgres.NewUserRepository(testPool(t))
	if _, err := repo.GetByLogin(ctx, "nobody"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetByLogin: %v", err)
	}
	if _, err := repo.GetByID(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetByID: %v", err)
	}
}
