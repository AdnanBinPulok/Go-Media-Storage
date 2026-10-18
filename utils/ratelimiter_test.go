package utils

import (
	"fmt"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// newLimiter wraps a limiter around a handler that always succeeds, using a
// unique scope per test so counters cannot leak between tests.
func newLimiterApp(scope string, requests int, window time.Duration) *fiber.App {
	app := fiber.New()
	app.Get("/", RateLimiter(RateLimitConfig{
		Requests: requests,
		Window:   window,
		Scope:    scope,
	}), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	return app
}

func TestRateLimiterBlocksAfterLimit(t *testing.T) {
	app := newLimiterApp("test-block", 3, time.Minute)

	for i := 0; i < 3; i++ {
		resp, err := app.Test(httptest.NewRequest("GET", "/", nil), -1)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, resp.StatusCode)
		}
	}

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil), -1)
	if err != nil {
		t.Fatalf("over-limit request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", resp.StatusCode)
	}
}

func TestRateLimiterScopesAreIndependent(t *testing.T) {
	app := fiber.New()

	// Two limiters with different scopes, one request each.
	for _, scope := range []string{"scope-a", "scope-b"} {
		scope := scope
		app.Get("/"+scope, RateLimiter(RateLimitConfig{
			Requests: 1,
			Window:   time.Minute,
			Scope:    scope,
		}), func(c *fiber.Ctx) error {
			return c.SendString("ok")
		})
	}

	for _, scope := range []string{"scope-a", "scope-b"} {
		resp, err := app.Test(httptest.NewRequest("GET", "/"+scope, nil), -1)
		if err != nil {
			t.Fatalf("%s: %v", scope, err)
		}
		resp.Body.Close()
		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("%s counted against the other scope: status = %d", scope, resp.StatusCode)
		}
	}
}

// TestRateLimiterConcurrent runs many goroutines through one limiter. Without
// the visitor mutex the counter loses increments and the limit is exceeded.
func TestRateLimiterConcurrent(t *testing.T) {
	const limit = 25
	app := newLimiterApp("test-concurrent", limit, time.Minute)

	var allowed atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := app.Test(httptest.NewRequest("GET", "/", nil), -1)
			if err != nil {
				t.Errorf("request: %v", err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode == fiber.StatusOK {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := allowed.Load(); got != limit {
		t.Errorf("allowed = %d, want exactly %d", got, limit)
	}
}

func TestRateLimiterResetsAfterWindow(t *testing.T) {
	app := newLimiterApp(fmt.Sprintf("test-reset-%d", time.Now().UnixNano()), 1, 30*time.Millisecond)

	first, err := app.Test(httptest.NewRequest("GET", "/", nil), -1)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	first.Body.Close()

	blocked, err := app.Test(httptest.NewRequest("GET", "/", nil), -1)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	blocked.Body.Close()
	if blocked.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", blocked.StatusCode)
	}

	time.Sleep(60 * time.Millisecond)

	after, err := app.Test(httptest.NewRequest("GET", "/", nil), -1)
	if err != nil {
		t.Fatalf("third: %v", err)
	}
	after.Body.Close()
	if after.StatusCode != fiber.StatusOK {
		t.Errorf("status after window = %d, want 200", after.StatusCode)
	}
}
