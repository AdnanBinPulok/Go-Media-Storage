package routes

import (
	"go-media-storage/modules"
	"go-media-storage/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

func StatusRoutes(router fiber.Router) {
	rateLimiter := utils.RateLimiter(utils.RateLimitConfig{
		Requests: 20,
		Window:   time.Second * 10,
	})

	router.Get("/status", rateLimiter, func(c *fiber.Ctx) error {
		return c.JSON(modules.GetPublicStatusData())
	})
}
