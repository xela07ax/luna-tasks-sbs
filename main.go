package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/xela07ax/luna-tasks-sbs/internal/config"
	"github.com/xela07ax/luna-tasks-sbs/internal/handler"
	"github.com/xela07ax/luna-tasks-sbs/internal/infra"
	"github.com/xela07ax/luna-tasks-sbs/internal/repository"
	"github.com/xela07ax/luna-tasks-sbs/internal/service"
	"github.com/xela07ax/luna-tasks-sbs/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development" // Значение по умолчанию для локального запуска
	}

	log, err := logger.InitLogger(env)
	if err != nil {
		panic("не удалось инициализировать логгер: " + err.Error())
	}
	// Заменяем глобальный логгер, если где-то используется zap.L()
	zap.ReplaceGlobals(log)
	defer log.Sync()

	log.Info("Сервис запущен", zap.String("env", env))
	log.Debug("Это дебаг сообщение, оно видно только локально")

	// 1. Загрузка конфигурации
	cfg, err := config.LoadConfig("configs")
	if err != nil {
		log.Fatal("Failed to load config", zap.Error(err))
	}

	// 2. Подключение к MySQL
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.DBName,
	)
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		log.Fatal("Failed to connect to MySQL", zap.Error(err))
	}
	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	// 3. Подключение к Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// 4. Инициализация слоев
	repo := repository.NewRepository(db, rdb)

	// Проверяем наличие ключей для JWT (RS256)
	if len(cfg.Auth.PrivateKey) == 0 || len(cfg.Auth.PublicKey) == 0 {
		log.Fatal("RSA keys for JWT are missing. Please provide certs/private.pem and certs/public.pem or set ENV vars.")
	}

	jwtManager, err := infra.NewJWTManager(cfg.Auth.PrivateKey, cfg.Auth.PublicKey, cfg.Auth.AccessTokenTTL)
	if err != nil {
		log.Fatal("Failed to init JWT Manager", zap.Error(err))
	}

	svc := service.NewService(repo, jwtManager, cfg.Auth.AccessTokenTTL, cfg.Auth.RefreshTokenTTL)
	h := handler.NewHandler(svc, log, cfg)

	router := h.InitRoutes()

	// 5. Запуск сервера метрик (отдельный порт для безопасности)
	if cfg.Metrics.Enabled {
		go func() {
			metricsMux := http.NewServeMux()
			metricsMux.Handle("/metrics", promhttp.Handler())
			metricsAddr := fmt.Sprintf(":%d", cfg.Metrics.Port)
			log.Info("Starting metrics server", zap.String("addr", metricsAddr))
			if err := http.ListenAndServe(metricsAddr, metricsMux); err != nil {
				log.Error("Metrics server failed", zap.Error(err))
			}
		}()
	}

	// 6. Запуск основного API сервера
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		log.Info("Starting API server", zap.Int("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed", zap.Error(err))
		}
	}()

	// 7. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exiting")
}
