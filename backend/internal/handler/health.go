package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthHandler struct {
	pool      *pgxpool.Pool
	startTime time.Time
}

func NewHealthHandler(pool *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{pool: pool, startTime: time.Now()}
}

func (h *HealthHandler) Health(c *fiber.Ctx) error {
	stat := h.pool.Stat()
	return c.JSON(fiber.Map{
		"status":         "ok",
		"uptime_seconds": int(time.Since(h.startTime).Seconds()),
		"db_pool": fiber.Map{
			"total_conns":    stat.TotalConns(),
			"idle_conns":     stat.IdleConns(),
			"acquired_conns": stat.AcquiredConns(),
			"max_conns":      stat.MaxConns(),
		},
	})
}

func (h *HealthHandler) Ready(c *fiber.Ctx) error {
	if err := h.pool.Ping(c.Context()); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not ready",
			"error":  "database unreachable",
		})
	}
	return c.JSON(fiber.Map{"status": "ready"})
}
