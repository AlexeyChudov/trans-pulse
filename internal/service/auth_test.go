package service_test

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/service"
)

func newAuth(users *fakeUsers) *service.AuthService {
	return service.NewAuthService(users, fakeTokens{}, service.WithBcryptCost(bcrypt.MinCost))
}

func TestRegisterCreatesDispatcherWithHashedPassword(t *testing.T) {
	users := newFakeUsers()
	u, err := newAuth(users).Register(ctx, "ivan", "correct-horse")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if u.Role != domain.RoleDispatcher || !u.IsActive {
		t.Fatalf("ожидался активный DISPATCHER, получено %+v", u)
	}
	stored := users.byID[u.ID]
	if stored.PasswordHash == "correct-horse" || bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("correct-horse")) != nil {
		t.Fatal("пароль должен храниться только как bcrypt-хэш")
	}
}

func TestRegisterRejectsDuplicateLogin(t *testing.T) {
	a := newAuth(newFakeUsers())
	if _, err := a.Register(ctx, "ivan", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Register(ctx, "ivan", "another-password"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ожидалось ErrConflict, получено %v", err)
	}
}

func TestRegisterValidatesInput(t *testing.T) {
	a := newAuth(newFakeUsers())
	cases := map[string][2]string{
		"короткий логин":   {"ab", "correct-horse"},
		"пробелы в логине": {"iv an", "correct-horse"},
		"длинный логин":    {strings.Repeat("a", 101), "correct-horse"},
		"короткий пароль":  {"ivan", "short"},
		"длинный пароль":   {"ivan", strings.Repeat("p", 73)},
	}
	for name, c := range cases {
		if _, err := a.Register(ctx, c[0], c[1]); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: ожидалось ErrInvalidInput, получено %v", name, err)
		}
	}
}

func TestCreateUserRejectsUnknownRole(t *testing.T) {
	if _, err := newAuth(newFakeUsers()).CreateUser(ctx, "boss", "correct-horse", domain.Role("ROOT")); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("ожидалось ErrInvalidInput, получено %v", err)
	}
}

func TestCreateUserCanCreateAdmin(t *testing.T) {
	u, err := newAuth(newFakeUsers()).CreateUser(ctx, "boss", "correct-horse", domain.RoleAdmin)
	if err != nil || u.Role != domain.RoleAdmin {
		t.Fatalf("получено %+v, %v", u, err)
	}
}

func TestLoginReturnsToken(t *testing.T) {
	a := newAuth(newFakeUsers())
	registered, _ := a.Register(ctx, "ivan", "correct-horse")

	token, u, err := a.Login(ctx, "ivan", "correct-horse")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token != "token-1-DISPATCHER" || u.ID != registered.ID {
		t.Fatalf("получено token=%q user=%+v", token, u)
	}
}

func TestLoginFailuresAreUnauthorized(t *testing.T) {
	users := newFakeUsers()
	a := newAuth(users)
	a.Register(ctx, "ivan", "correct-horse")
	a.Register(ctx, "blocked", "correct-horse")
	u := users.byID[2]
	u.IsActive = false
	users.byID[2] = u

	cases := map[string][2]string{
		"неверный пароль":          {"ivan", "wrong-password"},
		"неизвестный логин":        {"nobody", "correct-horse"},
		"отключённый пользователь": {"blocked", "correct-horse"},
	}
	for name, c := range cases {
		if _, _, err := a.Login(ctx, c[0], c[1]); !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("%s: ожидалось ErrUnauthorized, получено %v", name, err)
		}
	}
}

func TestMeReturnsUser(t *testing.T) {
	a := newAuth(newFakeUsers())
	registered, _ := a.Register(ctx, "ivan", "correct-horse")
	u, err := a.Me(ctx, registered.ID)
	if err != nil || u.Login != "ivan" {
		t.Fatalf("получено %+v, %v", u, err)
	}
	if _, err := a.Me(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ожидалось ErrNotFound, получено %v", err)
	}
}
