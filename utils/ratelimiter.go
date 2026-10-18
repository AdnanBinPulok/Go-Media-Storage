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
	// mu guards count and reset. Fiber serves requests concurrently, so
	// read-modify-write on the counter has to be serialized or the limit leaks.
	mu    sync.Mutex
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

		v.mu.Lock()
		if now.After(v.reset) {
			v.count = 0
			v.reset = now.Add(cfg.Window)
		}
		limited := v.count >= cfg.Requests
		if !limited {
			v.count++
		}
		v.mu.Unlock()

		// c.Next() must not run under the visitor lock: it would serialize every
		// request from this IP behind the handler.
		if limited {
			// 🔥 let error handler decide
			return fiber.NewError(
				fiber.StatusTooManyRequests,
				"Rate limit exceeded",
			)
		}

		return c.Next()
	}
}
