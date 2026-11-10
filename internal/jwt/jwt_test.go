package jwt_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	golangjwt "github.com/golang-jwt/jwt/v5"

	"github.com/AlexeyChudov/trans-pulse/internal/jwt"
)

const secret = "0123456789abcdef0123456789abcdef"

func newManager(t *testing.T, ttl time.Duration, opts ...jwt.Option) *jwt.Manager {
	t.Helper()
	m, err := jwt.NewManager(secret, ttl, opts...)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

func TestNewManagerRejectsShortSecret(t *testing.T) {
	if _, err := jwt.NewManager("short", time.Hour); err == nil {
		t.Fatal("ожидалась ошибка для короткого секрета")
	}
}

func TestNewManagerRejectsNonPositiveTTL(t *testing.T) {
	if _, err := jwt.NewManager(secret, 0); err == nil {
		t.Fatal("ожидалась ошибка для нулевого срока жизни")
	}
}

func TestGenerateThenParseReturnsClaims(t *testing.T) {
	m := newManager(t, time.Hour)

	token, err := m.Generate(42, "ADMIN")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	claims, err := m.Parse(token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.UserID != 42 || claims.Role != "ADMIN" {
		t.Fatalf("получено %+v, ожидалось UserID=42 Role=ADMIN", claims)
	}
}

func TestParseExpiredToken(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	issuer := newManager(t, time.Minute, jwt.WithClock(func() time.Time { return now }))
	token, err := issuer.Generate(1, "DISPATCHER")
	if err != nil {
		t.Fatal(err)
	}

	later := newManager(t, time.Minute, jwt.WithClock(func() time.Time { return now.Add(2 * time.Minute) }))
	if _, err := later.Parse(token); !errors.Is(err, jwt.ErrExpiredToken) {
		t.Fatalf("ожидалось ErrExpiredToken, получено %v", err)
	}
}

func TestParseTokenSignedWithOtherSecret(t *testing.T) {
	other, err := jwt.NewManager("ffffffffffffffffffffffffffffffff", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, _ := other.Generate(1, "ADMIN")

	if _, err := newManager(t, time.Hour).Parse(token); !errors.Is(err, jwt.ErrInvalidToken) {
		t.Fatalf("ожидалось ErrInvalidToken, получено %v", err)
	}
}

func TestParseTamperedToken(t *testing.T) {
	m := newManager(t, time.Hour)
	token, _ := m.Generate(1, "DISPATCHER")
	parts := strings.Split(token, ".")
	parts[1] = parts[1][:len(parts[1])-2] + "AA" // подменяем часть полезной нагрузки

	if _, err := m.Parse(strings.Join(parts, ".")); !errors.Is(err, jwt.ErrInvalidToken) {
		t.Fatalf("ожидалось ErrInvalidToken, получено %v", err)
	}
}

func TestParseRejectsUnsignedToken(t *testing.T) {
	unsigned := golangjwt.NewWithClaims(golangjwt.SigningMethodNone, golangjwt.MapClaims{
		"sub": "1", "role": "ADMIN", "exp": time.Now().Add(time.Hour).Unix(),
	})
	token, err := unsigned.SignedString(golangjwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := newManager(t, time.Hour).Parse(token); !errors.Is(err, jwt.ErrInvalidToken) {
		t.Fatalf("токен без подписи должен отклоняться, получено %v", err)
	}
}

func TestParseGarbage(t *testing.T) {
	if _, err := newManager(t, time.Hour).Parse("not-a-token"); !errors.Is(err, jwt.ErrInvalidToken) {
		t.Fatalf("ожидалось ErrInvalidToken, получено %v", err)
	}
}
