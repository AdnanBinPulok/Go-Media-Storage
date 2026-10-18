package utils

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type RateLimitConfig struct {
	Requests int
	Window   time.Duration

	// Scope separates counters per limiter. Without it every limiter in the app
	// shares one counter keyed by IP, so a dashboard page view would count
	// against the avatar route.
	Scope string
}

type visitor struct {
	count int
	reset time.Time
}

var visitors sync.Map

func RateLimiter(cfg RateLimitConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {

		ip := c.IP()
		if cfg.Scope != "" {
			ip = cfg.Scope + "|" + ip
		}
		now := time.Now()

		vAny, _ := visitors.LoadOrStore(ip, &visitor{
			count: 0,
			reset: now.Add(cfg.Window),
		})
		v := vAny.(*visitor)

		if now.After(v.reset) {
			v.count = 0
			v.reset = now.Add(cfg.Window)
		}

		if v.count >= cfg.Requests {
			// 🔥 let error handler decide
			return fiber.NewError(
				fiber.StatusTooManyRequests,
				"Rate limit exceeded",
			)
		}

		v.count++
		return c.Next()
	}
}
