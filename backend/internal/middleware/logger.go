package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
)

func RequestLogger(logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		duration := time.Since(start)
		status := c.Response().StatusCode()

		attrs := []slog.Attr{
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", status),
			slog.Int64("duration_ms", duration.Milliseconds()),
			slog.String("ip", c.IP()),
		}

		if reqID, ok := c.Locals(RequestIDKey).(string); ok {
			attrs = append(attrs, slog.String("request_id", reqID))
		}
		if userID, ok := c.Locals("user_id").(string); ok {
			attrs = append(attrs, slog.String("user_id", userID))
		}

		logArgs := make([]any, len(attrs))
		for i, a := range attrs {
			logArgs[i] = a
		}

		if status >= 500 {
			logger.Error("request completed", logArgs...)
		} else if status >= 400 {
			logger.Warn("request completed", logArgs...)
		} else {
			logger.Info("request completed", logArgs...)
		}

		return err
	}
}
