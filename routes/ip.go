package routes

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

func IpRoutes(router fiber.Router) {
	router.Get("/ip", func(c *fiber.Ctx) error {
		ip := c.IP()
		fmt.Printf("Client IP: %s\n", ip)
		return c.JSON(fiber.Map{"status": "ok", "message": "client IP retrieved", "ip": ip})
	})
}
