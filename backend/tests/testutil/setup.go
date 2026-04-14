package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	appHandler "github.com/taskflow/backend/internal/handler"
	"github.com/taskflow/backend/internal/apperror"
	"github.com/taskflow/backend/internal/config"
	"github.com/taskflow/backend/internal/middleware"
	"github.com/taskflow/backend/internal/repository"
	"github.com/taskflow/backend/internal/service"
)

type TestApp struct {
	App    *fiber.App
	Pool   *pgxpool.Pool
	Config *config.Config
}

func SetupTestApp(t *testing.T) *TestApp {
	t.Helper()

	os.Setenv("APP_ENV", "test")
	os.Setenv("DB_HOST", getEnvOrDefault("TEST_DB_HOST", "localhost"))
	os.Setenv("DB_PORT", getEnvOrDefault("TEST_DB_PORT", "5433"))
	os.Setenv("DB_USER", getEnvOrDefault("TEST_DB_USER", "taskflow"))
	os.Setenv("DB_PASSWORD", getEnvOrDefault("TEST_DB_PASSWORD", "taskflow_secret"))
	os.Setenv("DB_NAME", getEnvOrDefault("TEST_DB_NAME", "taskflow_test"))
	os.Setenv("DB_SSL_MODE", "disable")
	os.Setenv("JWT_SECRET", "test-jwt-secret-at-least-16-chars")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL())
	if err != nil {
		t.Skipf("database not available, skipping: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("database not reachable, skipping: %v", err)
	}

	cleanDB(t, pool)
	setupSchema(t, pool)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

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

	authHandler := appHandler.NewAuthHandler(authService, cfg)
	projectHandler := appHandler.NewProjectHandler(projectService)
	taskHandler := appHandler.NewTaskHandler(taskService)
	webhookHandler := appHandler.NewWebhookHandler(webhookRepo)
	healthHandler := appHandler.NewHealthHandler(pool)

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			if ae, ok := apperror.AsAppError(err); ok {
				resp := fiber.Map{"error": ae.Msg}
				if ae.Fields != nil {
					resp["fields"] = ae.Fields
				}
				return c.Status(ae.Code).JSON(resp)
			}
			if fe, ok := err.(*fiber.Error); ok {
				return c.Status(fe.Code).JSON(fiber.Map{"error": fe.Message})
			}
			return c.Status(500).JSON(fiber.Map{"error": "internal server error"})
		},
	})

	app.Use(middleware.RequestID())

	health := app.Group("/health")
	health.Get("/", healthHandler.Health)

	authGroup := app.Group("/api/v1/auth")
	authGroup.Post("/register", authHandler.Register)
	authGroup.Post("/login", authHandler.Login)
	authGroup.Post("/refresh", authHandler.Refresh)
	authGroup.Post("/logout", authHandler.Logout)

	api := app.Group("/api/v1")
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

	webhooks := api.Group("/webhooks")
	webhooks.Post("/", webhookHandler.Create)
	webhooks.Get("/", webhookHandler.List)
	webhooks.Delete("/:id", webhookHandler.Delete)

	t.Cleanup(func() {
		sseBroker.Shutdown()
		pool.Close()
	})

	return &TestApp{App: app, Pool: pool, Config: cfg}
}

func cleanDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	tables := []string{"webhook_delivery_log", "webhook_subscriptions", "refresh_tokens", "tasks", "projects", "users"}
	for _, table := range tables {
		_, _ = pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table))
	}
	_, _ = pool.Exec(ctx, "DROP TYPE IF EXISTS task_status CASCADE")
	_, _ = pool.Exec(ctx, "DROP TYPE IF EXISTS task_priority CASCADE")
}

func setupSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	ddl := []string{
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(100) NOT NULL,
			email VARCHAR(255) NOT NULL,
			password VARCHAR(255) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT users_email_unique UNIQUE (email)
		)`,
		`DO $$ BEGIN CREATE TYPE task_status AS ENUM ('todo','in_progress','done'); EXCEPTION WHEN duplicate_object THEN null; END $$`,
		`DO $$ BEGIN CREATE TYPE task_priority AS ENUM ('low','medium','high'); EXCEPTION WHEN duplicate_object THEN null; END $$`,
		`CREATE TABLE IF NOT EXISTS projects (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			description TEXT,
			owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			title VARCHAR(255) NOT NULL,
			description TEXT,
			status task_status NOT NULL DEFAULT 'todo',
			priority task_priority NOT NULL DEFAULT 'medium',
			project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			creator_id UUID NOT NULL REFERENCES users(id),
			assignee_id UUID REFERENCES users(id) ON DELETE SET NULL,
			due_date DATE,
			version INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS refresh_tokens (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash VARCHAR(64) NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS webhook_subscriptions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			url TEXT NOT NULL,
			secret VARCHAR(256) NOT NULL,
			event_types TEXT[] NOT NULL,
			project_ids UUID[] NOT NULL,
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT webhook_subs_unique UNIQUE (user_id, url)
		)`,
		`CREATE TABLE IF NOT EXISTS webhook_delivery_log (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			subscription_id UUID NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
			event_type VARCHAR(50) NOT NULL,
			payload JSONB NOT NULL,
			response_status INTEGER,
			response_body TEXT,
			attempt INTEGER NOT NULL DEFAULT 1,
			success BOOLEAN NOT NULL DEFAULT false,
			error_message TEXT,
			delivered_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	}

	for _, stmt := range ddl {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("schema setup failed: %v\nStatement: %s", err, stmt)
		}
	}
}

func DoRequest(app *fiber.App, method, path string, body interface{}, token string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("x-access-token", token)
	}

	return app.Test(req, -1)
}

func DoRequestWithCookie(app *fiber.App, method, path string, body interface{}, cookies []*http.Cookie) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	return app.Test(req, -1)
}

func ParseJSON(resp *http.Response) (map[string]interface{}, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w (body: %s)", err, string(body))
	}
	return result, nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// RegisterAndLogin creates a test user and returns the access token
func RegisterAndLogin(t *testing.T, app *fiber.App) (string, string) {
	t.Helper()
	email := fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())
	body := map[string]string{
		"name":     "Test User",
		"email":    email,
		"password": "password123",
	}

	resp, err := DoRequest(app, "POST", "/api/v1/auth/register", body, "")
	if err != nil {
		t.Fatalf("register request failed: %v", err)
	}
	if resp.StatusCode != 201 {
		data, _ := ParseJSON(resp)
		t.Fatalf("register returned %d: %v", resp.StatusCode, data)
	}

	data, err := ParseJSON(resp)
	if err != nil {
		t.Fatalf("failed to parse register response: %v", err)
	}

	token, ok := data["access_token"].(string)
	if !ok || token == "" {
		t.Fatal("no access_token in register response")
	}

	user := data["user"].(map[string]interface{})
	userID := user["id"].(string)

	return token, userID
}
