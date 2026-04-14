package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/taskflow/backend/internal/config"
)

func CORS(cfg *config.Config) fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     strings.Join(cfg.CORSAllowedOrigins, ","),
		AllowMethods:     "GET,POST,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "x-access-token,Content-Type,Authorization,X-Request-Id",
		ExposeHeaders:    "X-Request-Id",
		AllowCredentials: true,
		MaxAge:           86400,
	})
}
