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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/denysdovzhenko/url-shortener/internal/cache"
	"github.com/denysdovzhenko/url-shortener/internal/config"
	"github.com/denysdovzhenko/url-shortener/internal/handler"
	"github.com/denysdovzhenko/url-shortener/internal/repository"
	"github.com/denysdovzhenko/url-shortener/internal/service"
	"github.com/denysdovzhenko/url-shortener/web"
)

// version is stamped at link time with -X main.version (see the Dockerfile and
// the Makefile). It stays "dev" for `go run` and air builds.
var version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return err
	}
	logger.Info("connected to postgres")

	redisCache := cache.NewRedisCache(cfg.RedisAddr, cfg.RedisDB, cfg.RedisPassword, cfg.RedisTLS)
	defer func() {
		if err := redisCache.Close(); err != nil {
			logger.Error("closing redis", "error", err)
		}
	}()

	if err := redisCache.Ping(ctx); err != nil {
		return err
	}
	logger.Info("connected to redis")

	store := repository.NewStore(pool)
	urlRepo := repository.NewURLRepository(store)

	shortenerSvc := service.NewShortenerService(
		urlRepo,
		redisCache,
		cfg.BaseURL,
		cfg.CodeLength,
		cfg.CacheTTL,
		logger,
	)

	urlHandler := handler.NewURLHandler(shortenerSvc, logger, version)
	staticHandler := handler.NewStaticHandler(web.Static())
	router := handler.NewRouter(urlHandler, staticHandler)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting server", "port", cfg.HTTPPort, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}
