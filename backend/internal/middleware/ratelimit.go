package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type RateLimitConfig struct {
	Max       int
	Window    time.Duration
	KeyFunc   func(c *fiber.Ctx) string
}

type entry struct {
	count     int
	expiresAt time.Time
}

type rateLimiter struct {
	entries sync.Map
	stop    chan struct{}
}

var globalLimiter = &rateLimiter{
	stop: make(chan struct{}),
}

func init() {
	go globalLimiter.cleanup()
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			rl.entries.Range(func(key, value any) bool {
				e := value.(*entry)
				if now.After(e.expiresAt) {
					rl.entries.Delete(key)
				}
				return true
			})
		case <-rl.stop:
			return
		}
	}
}

func StopRateLimiterCleanup() {
	close(globalLimiter.stop)
}

func RateLimit(cfg RateLimitConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := cfg.KeyFunc(c)
		now := time.Now()

		val, _ := globalLimiter.entries.LoadOrStore(key, &entry{
			count:     0,
			expiresAt: now.Add(cfg.Window),
		})
		e := val.(*entry)

		if now.After(e.expiresAt) {
			e.count = 0
			e.expiresAt = now.Add(cfg.Window)
		}

		e.count++

		if e.count > cfg.Max {
			retryAfter := int(time.Until(e.expiresAt).Seconds()) + 1
			c.Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": fmt.Sprintf("too many requests, retry after %d seconds", retryAfter),
			})
		}

		return c.Next()
	}
}
