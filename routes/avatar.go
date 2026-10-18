package routes

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go-media-storage/pages"
	"go-media-storage/settings"
	"go-media-storage/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// AvatarRoutes mounts public avatar helpers under /api.
//
// GET /api/avatar/random mints a UUID and returns a URL to /avatar/:uuid.
// It does not render or store anything — the image route owns that.
func AvatarRoutes(router fiber.Router) {
	rateLimiter := utils.RateLimiter(utils.RateLimitConfig{
		Requests: 300,
		Window:   time.Second * 10,
		Scope:    "avatar",
	})

	router.Get("/avatar/random", rateLimiter, randomAvatar)
}

func randomAvatar(c *fiber.Ctx) error {
	size := pages.AvatarDefaultSize()
	if raw := c.Query("size"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  "error",
				"message": "Invalid size",
			})
		}
		size = pages.ClampAvatarSize(parsed)
	}

	id := uuid.NewString()
	url := avatarURL(id, size)

	c.Set(fiber.HeaderCacheControl, "no-store")

	return c.JSON(fiber.Map{
		"status": "ok",
		"uuid":   id,
		"size":   size,
		"url":    url,
	})
}

// avatarURL builds an absolute image URL from BASE_URL.
func avatarURL(id string, size int) string {
	base := strings.TrimRight(settings.AppInfoInstance.BaseURL(), "/")
	return fmt.Sprintf("%s/avatar/%s?size=%d", base, id, size)
}
