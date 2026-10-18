package pages

import (
	"fmt"
	"go-media-storage/avatar_generator"
	"go-media-storage/settings"
	"go-media-storage/utils"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Get example: /avatar/550e8400-e29b-41d4-a716-446655440000?size=256
//
// The avatar is a pure function of (uuid, size), so the response is cacheable
// for a long time and the ETag never changes for a given pair.
func AvatarPage(router fiber.Router) {
	rateLimiter := utils.RateLimiter(utils.RateLimitConfig{
		Requests: 300,
		Window:   time.Second * 10,
		Scope:    "avatar",
	})

	router.Get("/avatar/:uuid", rateLimiter, func(c *fiber.Ctx) error {
		// Every valid UUID spelling normalizes to one lowercase canonical form,
		// so it maps to one avatar and one cache entry.
		id, ok := utils.NormalizeUUID(c.Params("uuid"))
		if !ok {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid UUID")
		}

		size := AvatarDefaultSize()
		if raw := c.Query("size"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).SendString("Invalid size")
			}
			size = ClampAvatarSize(parsed)
		}

		// Oversized renders are gated. Queue time is not generation time, so it
		// is spent outside the measurement below.
		if size > heavySize {
			if !acquireHeavySlot() {
				return fiber.NewError(fiber.StatusServiceUnavailable, "Too many large avatar requests")
			}
			defer releaseHeavySlot()
		}

		genStart := time.Now()

		png, hit, err := avatarStore().PNG(id, size, avatar_generator.DefaultPaddingPct)
		genTime := time.Since(genStart)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Failed to generate avatar")
		}

		setAvatarCacheHeaders(c, id, size)

		if hit {
			c.Set("X-Avatar-Cache", "HIT")
		} else {
			c.Set("X-Avatar-Cache", "MISS")
		}

		// Conditional request: the browser already has these exact bytes.
		if c.Fresh() {
			c.Status(fiber.StatusNotModified)
			return nil
		}

		c.Set(fiber.HeaderContentType, "image/png")
		c.Set("X-Content-Type-Options", "nosniff")

		c.Set("X-Avatar-Generation-Time", fmt.Sprintf("%dms", genTime.Milliseconds()))

		return c.Send(png)
	})
}

// setAvatarCacheHeaders marks the response as public and long-lived.
//
// max-age and s-maxage cover browsers and generic proxies. The two CDN headers
// are what Cloudflare actually honours: Cloudflare-CDN-Cache-Control wins over
// CDN-Cache-Control, which wins over s-maxage. Setting them separately means the
// edge can keep an avatar for a day without changing how the client behaves.
func setAvatarCacheHeaders(c *fiber.Ctx, id string, size int) {
	ttl := settings.CachingConfigInstance.AvatarCacheTime
	if ttl <= 0 {
		ttl = 86400
	}

	c.Set(fiber.HeaderCacheControl, fmt.Sprintf("public, max-age=%d, s-maxage=%d, immutable", ttl, ttl))
	c.Set("CDN-Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", ttl))
	c.Set("Cloudflare-CDN-Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", ttl))
	c.Set(fiber.HeaderETag, avatar_generator.ETag(id, size, avatar_generator.DefaultPaddingPct))
}
