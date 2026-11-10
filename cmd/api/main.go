// Команда api запускает REST API trans-pulse.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AlexeyChudov/trans-pulse/internal/config"
	httpapi "github.com/AlexeyChudov/trans-pulse/internal/http"
	"github.com/AlexeyChudov/trans-pulse/internal/jwt"
	mongorepo "github.com/AlexeyChudov/trans-pulse/internal/repository/mongo"
	"github.com/AlexeyChudov/trans-pulse/internal/repository/postgres"
	"github.com/AlexeyChudov/trans-pulse/internal/service"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("приложение остановлено с ошибкой", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	mongoClient, err := mongorepo.Connect(ctx, cfg.MongoURI)
	if err != nil {
		return err
	}
	defer func() { _ = mongoClient.Disconnect(context.Background()) }()
	mongoDB := mongoClient.Database(cfg.MongoDB)
	// Схемы валидации и индексы применяются при каждом старте (операция идемпотентна).
	if err := mongorepo.EnsureSchema(ctx, mongoDB); err != nil {
		return err
	}

	tokens, err := jwt.NewManager(cfg.JWTSecret, cfg.JWTTTL)
	if err != nil {
		return err
	}

	routes := postgres.NewRouteRepository(pool)
	vehicles := postgres.NewVehicleRepository(pool)
	trips := postgres.NewTripRepository(pool)
	incidents := postgres.NewIncidentRepository(pool)
	handler := httpapi.NewRouter(httpapi.Deps{
		Auth:      service.NewAuthService(postgres.NewUserRepository(pool), tokens),
		Routes:    service.NewRouteService(routes),
		Vehicles:  service.NewVehicleService(vehicles),
		Trips:     service.NewTripService(trips, routes, vehicles, nil),
		Incidents: service.NewIncidentService(incidents, nil),
		Telemetry: service.NewTelemetryService(mongorepo.NewTelemetryRepository(mongoDB), vehicles, nil),
		Journals:  service.NewJournalService(mongorepo.NewJournalRepository(mongoDB), trips, nil),
		Cases:     service.NewCaseService(mongorepo.NewCaseRepository(mongoDB), incidents, trips, nil),
		Geo:       service.NewGeoService(mongorepo.NewStopPointRepository(mongoDB), mongorepo.NewRouteProfileRepository(mongoDB), routes),
		Tokens:    tokens,
		Logger:    log,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("сервер запущен", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Info("получен сигнал остановки, завершаем работу")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
