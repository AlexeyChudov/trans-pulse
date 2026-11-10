// Команда createadmin создаёт первого администратора: go run ./cmd/createadmin -login boss
// Пароль берётся из переменной ADMIN_PASSWORD (чтобы не попадал в историю командной строки).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/AlexeyChudov/trans-pulse/internal/domain"
	"github.com/AlexeyChudov/trans-pulse/internal/repository/postgres"
	"github.com/AlexeyChudov/trans-pulse/internal/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

func run() error {
	login := flag.String("login", "", "логин администратора")
	flag.Parse()
	password := os.Getenv("ADMIN_PASSWORD")
	dsn := os.Getenv("DATABASE_URL")
	if *login == "" || password == "" || dsn == "" {
		return errors.New("нужны флаг -login и переменные окружения ADMIN_PASSWORD и DATABASE_URL")
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Токены этой команде не нужны: пользователь только создаётся.
	auth := service.NewAuthService(postgres.NewUserRepository(pool), nil)
	u, err := auth.CreateUser(ctx, *login, password, domain.RoleAdmin)
	if err != nil {
		return err
	}
	fmt.Printf("администратор создан: id=%d login=%s\n", u.ID, u.Login)
	return nil
}
