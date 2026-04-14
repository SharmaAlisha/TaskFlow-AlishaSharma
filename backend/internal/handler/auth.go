package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/taskflow/backend/internal/config"
	"github.com/taskflow/backend/internal/dto"
	"github.com/taskflow/backend/internal/service"
	"github.com/taskflow/backend/internal/validator"
)

type AuthHandler struct {
	authService *service.AuthService
	cfg         *config.Config
}

func NewAuthHandler(authService *service.AuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{authService: authService, cfg: cfg}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req dto.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := validator.Validate(&req); err != nil {
		return err
	}

	resp, refreshToken, err := h.authService.Register(c.Context(), req)
	if err != nil {
		return err
	}

	h.setRefreshCookie(c, refreshToken)
	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req dto.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := validator.Validate(&req); err != nil {
		return err
	}

	resp, refreshToken, err := h.authService.Login(c.Context(), req)
	if err != nil {
		return err
	}

	h.setRefreshCookie(c, refreshToken)
	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	rawToken := c.Cookies("refresh_token")
	if rawToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	resp, newRefreshToken, err := h.authService.Refresh(c.Context(), rawToken)
	if err != nil {
		return err
	}

	h.setRefreshCookie(c, newRefreshToken)
	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	rawToken := c.Cookies("refresh_token")
	if rawToken != "" {
		_ = h.authService.Logout(c.Context(), rawToken)
	}

	h.clearRefreshCookie(c)
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *AuthHandler) setRefreshCookie(c *fiber.Ctx, token string) {
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/v1/auth",
		HTTPOnly: true,
		Secure:   h.cfg.AppEnv == "production",
		SameSite: "Lax",
		Expires:  time.Now().Add(h.cfg.JWTRefreshExpiry),
	})
}

func (h *AuthHandler) clearRefreshCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth",
		HTTPOnly: true,
		Secure:   h.cfg.AppEnv == "production",
		SameSite: "Lax",
		Expires:  time.Now().Add(-1 * time.Hour),
	})
}
