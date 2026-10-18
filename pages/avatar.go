package pages

import (
	"github.com/gofiber/fiber/v2"
)

// Get example: /file/view/abc123def456.jpg
func AvatarPage(router fiber.Router) {
	router.Get("/avatar/:uuid", func(c *fiber.Ctx) error {
		uuid := c.Params("uuid")
		return c.SendString(uuid)
	})
}
