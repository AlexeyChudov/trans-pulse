package service

import (
	"context"
	"errors"
	"regexp"

	"golang.org/x/crypto/bcrypt"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 72 // ограничение bcrypt
)

var loginPattern = regexp.MustCompile(`^[A-Za-z0-9._@-]{3,100}$`)

// TokenIssuer выпускает токен доступа (реализует internal/jwt.Manager).
type TokenIssuer interface {
	Generate(userID int64, role string) (string, error)
}

type AuthService struct {
	users     domain.UserRepository
	tokens    TokenIssuer
	cost      int
	dummyHash []byte // для выравнивания времени ответа, если логина не существует
}

type AuthOption func(*AuthService)

func WithBcryptCost(cost int) AuthOption { return func(s *AuthService) { s.cost = cost } }

func NewAuthService(users domain.UserRepository, tokens TokenIssuer, opts ...AuthOption) *AuthService {
	s := &AuthService{users: users, tokens: tokens, cost: bcrypt.DefaultCost}
	for _, opt := range opts {
		opt(s)
	}
	s.dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), s.cost)
	return s
}

// Register создаёт диспетчера (публичная регистрация).
func (s *AuthService) Register(ctx context.Context, login, password string) (domain.User, error) {
	return s.CreateUser(ctx, login, password, domain.RoleDispatcher)
}

// CreateUser создаёт пользователя с заданной ролью (используется и командой createadmin).
func (s *AuthService) CreateUser(ctx context.Context, login, password string, role domain.Role) (domain.User, error) {
	if !loginPattern.MatchString(login) {
		return domain.User{}, invalid("логин: 3–100 символов (буквы, цифры, . _ @ -)")
	}
	if len(password) < minPasswordLen || len(password) > maxPasswordLen {
		return domain.User{}, invalid("пароль: от %d до %d символов", minPasswordLen, maxPasswordLen)
	}
	if !role.IsValid() {
		return domain.User{}, invalid("неизвестная роль")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return domain.User{}, err
	}
	u := domain.User{Login: login, PasswordHash: string(hash), Role: role, IsActive: true}
	if err := s.users.Create(ctx, &u); err != nil {
		return domain.User{}, err
	}
	return u, nil
}

// Login проверяет пароль и выпускает токен. Любая причина отказа возвращается как ErrUnauthorized.
func (s *AuthService) Login(ctx context.Context, login, password string) (string, domain.User, error) {
	u, err := s.users.GetByLogin(ctx, login)
	if errors.Is(err, domain.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return "", domain.User{}, domain.ErrUnauthorized
	}
	if err != nil {
		return "", domain.User{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil || !u.IsActive {
		return "", domain.User{}, domain.ErrUnauthorized
	}
	token, err := s.tokens.Generate(u.ID, string(u.Role))
	if err != nil {
		return "", domain.User{}, err
	}
	return token, u, nil
}

func (s *AuthService) Me(ctx context.Context, userID int64) (domain.User, error) {
	return s.users.GetByID(ctx, userID)
}
