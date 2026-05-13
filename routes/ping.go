package routes

import (
	"github.com/gofiber/fiber/v2"
)

func PingRoutes(router fiber.Router) {
	router.Get("/ping", func(c *fiber.Ctx) error {
		// return the ip as JSON
		return c.JSON(fiber.Map{"status": "ok", "message": "pong", "ip": c.IP()})
	})
}
