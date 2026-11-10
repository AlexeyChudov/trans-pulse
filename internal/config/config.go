// Package config читает настройки приложения из переменных окружения.
package config

import (
	"errors"
	"fmt"
	"time"
)

const (
	defaultHTTPAddr = ":8080"
	defaultJWTTTL   = time.Hour
	defaultMongoDB  = "trans_pulse"
	minSecretLen    = 32
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	JWTTTL      time.Duration
	HTTPAddr    string
	MongoURI    string
	MongoDB     string
}

// Load читает настройки через getenv (os.Getenv в приложении, словарь в тестах).
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DatabaseURL: getenv("DATABASE_URL"),
		JWTSecret:   getenv("JWT_SECRET"),
		HTTPAddr:    getenv("HTTP_ADDR"),
		MongoURI:    getenv("MONGO_URI"),
		MongoDB:     getenv("MONGO_DB"),
		JWTTTL:      defaultJWTTTL,
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = defaultHTTPAddr
	}
	if cfg.MongoDB == "" {
		cfg.MongoDB = defaultMongoDB
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("config: не задана переменная DATABASE_URL")
	}
	if cfg.MongoURI == "" {
		return Config{}, errors.New("config: не задана переменная MONGO_URI")
	}
	if cfg.JWTSecret == "" {
		return Config{}, errors.New("config: не задана переменная JWT_SECRET")
	}
	// Значение секрета в сообщения об ошибках не попадает.
	if len(cfg.JWTSecret) < minSecretLen {
		return Config{}, fmt.Errorf("config: JWT_SECRET должен быть не короче %d символов", minSecretLen)
	}
	if raw := getenv("JWT_TTL"); raw != "" {
		ttl, err := time.ParseDuration(raw)
		if err != nil || ttl <= 0 {
			return Config{}, fmt.Errorf("config: JWT_TTL должен быть положительной длительностью (например, 1h), получено %q", raw)
		}
		cfg.JWTTTL = ttl
	}
	return cfg, nil
}
