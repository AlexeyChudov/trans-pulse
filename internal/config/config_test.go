package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/config"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

const goodSecret = "0123456789abcdef0123456789abcdef"

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": goodSecret, "MONGO_URI": "mongodb://x"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.JWTTTL != time.Hour || cfg.MongoURI != "mongodb://x" || cfg.MongoDB != "trans_pulse" || cfg.DatabaseURL != "postgres://x" || cfg.JWTSecret != goodSecret {
		t.Fatalf("получено %+v", cfg)
	}
}

func TestLoadReadsCustomValues(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{
		"DATABASE_URL": "postgres://x", "JWT_SECRET": goodSecret, "HTTP_ADDR": ":9000", "JWT_TTL": "15m", "MONGO_URI": "mongodb://x", "MONGO_DB": "custom",
	}))
	if err != nil || cfg.HTTPAddr != ":9000" || cfg.JWTTTL != 15*time.Minute || cfg.MongoDB != "custom" {
		t.Fatalf("получено %+v, %v", cfg, err)
	}
}

func TestLoadRejectsMissingOrBadValues(t *testing.T) {
	cases := map[string]map[string]string{
		"нет DATABASE_URL": {"JWT_SECRET": goodSecret, "MONGO_URI": "mongodb://x"},
		"нет JWT_SECRET":   {"DATABASE_URL": "postgres://x", "MONGO_URI": "mongodb://x"},
		"нет MONGO_URI":    {"DATABASE_URL": "postgres://x", "JWT_SECRET": goodSecret},
		"короткий секрет":  {"DATABASE_URL": "postgres://x", "JWT_SECRET": "short", "MONGO_URI": "mongodb://x"},
		"плохой JWT_TTL":   {"DATABASE_URL": "postgres://x", "JWT_SECRET": goodSecret, "JWT_TTL": "tomorrow", "MONGO_URI": "mongodb://x"},
		"нулевой JWT_TTL":  {"DATABASE_URL": "postgres://x", "JWT_SECRET": goodSecret, "JWT_TTL": "0s", "MONGO_URI": "mongodb://x"},
	}
	for name, vars := range cases {
		if _, err := config.Load(env(vars)); err == nil {
			t.Errorf("%s: ожидалась ошибка", name)
		}
	}
}

func TestLoadErrorDoesNotLeakSecret(t *testing.T) {
	_, err := config.Load(env(map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": "short-secret-value"}))
	if err == nil || strings.Contains(err.Error(), "short-secret-value") {
		t.Fatalf("ошибка не должна содержать значение секрета: %v", err)
	}
}
