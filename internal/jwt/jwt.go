// Package jwt выпускает и проверяет токены доступа (HS256). Пакет не зависит от остальных слоёв приложения.
package jwt

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	golangjwt "github.com/golang-jwt/jwt/v5"
)

const (
	minSecretLen = 32
	issuer       = "trans-pulse"
)

var (
	ErrInvalidToken = errors.New("jwt: недействительный токен")
	ErrExpiredToken = errors.New("jwt: срок действия токена истёк")
)

// Claims — данные пользователя, которые хранятся в токене.
type Claims struct {
	UserID int64
	Role   string
}

type tokenClaims struct {
	Role string `json:"role"`
	golangjwt.RegisteredClaims
}

type Manager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

type Option func(*Manager)

// WithClock подменяет источник времени (для тестов).
func WithClock(now func() time.Time) Option {
	return func(m *Manager) { m.now = now }
}

func NewManager(secret string, ttl time.Duration, opts ...Option) (*Manager, error) {
	if len(secret) < minSecretLen {
		return nil, fmt.Errorf("jwt: секрет должен быть не короче %d символов", minSecretLen)
	}
	if ttl <= 0 {
		return nil, errors.New("jwt: срок жизни токена должен быть положительным")
	}
	m := &Manager{secret: []byte(secret), ttl: ttl, now: time.Now}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

func (m *Manager) Generate(userID int64, role string) (string, error) {
	now := m.now()
	claims := tokenClaims{
		Role: role,
		RegisteredClaims: golangjwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			Issuer:    issuer,
			IssuedAt:  golangjwt.NewNumericDate(now),
			ExpiresAt: golangjwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *Manager) Parse(token string) (Claims, error) {
	parsed, err := golangjwt.ParseWithClaims(token, &tokenClaims{},
		func(*golangjwt.Token) (any, error) { return m.secret, nil },
		golangjwt.WithValidMethods([]string{golangjwt.SigningMethodHS256.Alg()}),
		golangjwt.WithTimeFunc(m.now),
		golangjwt.WithIssuer(issuer),
		golangjwt.WithExpirationRequired(),
	)
	if err != nil {
		if errors.Is(err, golangjwt.ErrTokenExpired) {
			return Claims{}, ErrExpiredToken
		}
		return Claims{}, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(*tokenClaims)
	if !ok {
		return Claims{}, ErrInvalidToken
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	return Claims{UserID: id, Role: claims.Role}, nil
}
