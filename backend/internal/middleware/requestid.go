package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const RequestIDKey = "request_id"

func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		c.Locals(RequestIDKey, id)
		c.Set("X-Request-Id", id)
		return c.Next()
	}
}
