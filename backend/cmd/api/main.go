package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/taskflow/backend/internal/apperror"
	"github.com/taskflow/backend/internal/config"
	"github.com/taskflow/backend/internal/database"
	"github.com/taskflow/backend/internal/handler"
	"github.com/taskflow/backend/internal/middleware"
	"github.com/taskflow/backend/internal/repository"
	"github.com/taskflow/backend/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logLevel := parseLogLevel(cfg.LogLevel)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	logger.Info("connected to database", "host", cfg.DBHost, "db", cfg.DBName)

	userRepo := repository.NewUserRepository(pool)
	refreshRepo := repository.NewRefreshTokenRepository(pool)
	projectRepo := repository.NewProjectRepository(pool)
	taskRepo := repository.NewTaskRepository(pool)
	webhookRepo := repository.NewWebhookRepository(pool)

	sseBroker := service.NewSSEBroker(logger)
	webhookDispatcher := service.NewWebhookDispatcher(webhookRepo, cfg, logger)

	authService := service.NewAuthService(userRepo, refreshRepo, cfg, logger)
	projectService := service.NewProjectService(projectRepo, taskRepo, userRepo, sseBroker, webhookDispatcher, logger)
	taskService := service.NewTaskService(taskRepo, projectRepo, sseBroker, webhookDispatcher, logger)

	authHandler := handler.NewAuthHandler(authService, cfg)
	projectHandler := handler.NewProjectHandler(projectService)
	taskHandler := handler.NewTaskHandler(taskService)
	sseHandler := handler.NewSSEHandler(sseBroker, projectRepo, logger)
	webhookHandler := handler.NewWebhookHandler(webhookRepo)
	healthHandler := handler.NewHealthHandler(pool)

	app := fiber.New(fiber.Config{
		BodyLimit:    cfg.BodyLimitMB * 1024 * 1024,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
		ErrorHandler: globalErrorHandler(logger),
	})

	app.Use(middleware.Recover(logger))
	app.Use(middleware.RequestID())
	app.Use(middleware.RequestLogger(logger))
	app.Use(middleware.SecurityHeaders(cfg))
	app.Use(middleware.CORS(cfg))

	health := app.Group("/health")
	health.Get("/", healthHandler.Health)
	health.Get("/ready", healthHandler.Ready)

	auth := app.Group("/api/v1/auth")
	auth.Use(middleware.RateLimit(middleware.RateLimitConfig{
		Max:    cfg.RateLimitAuthMax,
		Window: cfg.RateLimitAuthWindow,
		KeyFunc: func(c *fiber.Ctx) string {
			return fmt.Sprintf("auth:%s", c.IP())
		},
	}))
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", authHandler.Refresh)
	auth.Post("/logout", authHandler.Logout)

	api := app.Group("/api/v1")
	api.Use(middleware.RateLimit(middleware.RateLimitConfig{
		Max:    cfg.RateLimitAPIMax,
		Window: cfg.RateLimitAPIWindow,
		KeyFunc: func(c *fiber.Ctx) string {
			if uid, ok := c.Locals("user_id").(string); ok {
				return fmt.Sprintf("api:%s", uid)
			}
			return fmt.Sprintf("api:%s", c.IP())
		},
	}))
	api.Use(middleware.Auth(cfg.JWTSecret))

	projects := api.Group("/projects")
	projects.Get("/", projectHandler.List)
	projects.Post("/", projectHandler.Create)
	projects.Get("/:id", projectHandler.GetByID)
	projects.Patch("/:id", projectHandler.Update)
	projects.Delete("/:id", projectHandler.Delete)
	projects.Get("/:id/stats", projectHandler.Stats)
	projects.Get("/:id/tasks", taskHandler.List)
	projects.Post("/:id/tasks", taskHandler.Create)

	tasks := api.Group("/tasks")
	tasks.Patch("/:id", taskHandler.Update)
	tasks.Delete("/:id", taskHandler.Delete)

	sse := api.Group("/sse")
	sse.Get("/events", sseHandler.Stream)

	webhooks := api.Group("/webhooks")
	webhooks.Post("/", webhookHandler.Create)
	webhooks.Get("/", webhookHandler.List)
	webhooks.Delete("/:id", webhookHandler.Delete)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		addr := fmt.Sprintf("%s:%s", cfg.AppHost, cfg.AppPort)
		logger.Info("starting server", "addr", addr)
		if err := app.Listen(addr); err != nil {
			logger.Error("server listen error", "error", err)
		}
	}()

	<-quit
	logger.Info("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	sseBroker.Shutdown()
	webhookDispatcher.Shutdown(shutdownCtx)
	middleware.StopRateLimiterCleanup()
	pool.Close()

	logger.Info("server stopped cleanly")
}

func globalErrorHandler(logger *slog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		if ae, ok := apperror.AsAppError(err); ok {
			if ae.Inner != nil {
				logger.Error("app error", "error", ae.Inner, "path", c.Path())
			}
			resp := fiber.Map{"error": ae.Msg}
			if ae.Fields != nil {
				resp["fields"] = ae.Fields
			}
			return c.Status(ae.Code).JSON(resp)
		}

		if fe, ok := err.(*fiber.Error); ok {
			return c.Status(fe.Code).JSON(fiber.Map{"error": fe.Message})
		}

		logger.Error("unhandled error", "error", err, "path", c.Path())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
