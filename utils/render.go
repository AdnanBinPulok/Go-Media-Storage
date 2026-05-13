package utils

import (
	"github.com/gofiber/fiber/v2"
)

func RenderPage(c *fiber.Ctx, view string, data fiber.Map) error {
	if data == nil {
		data = fiber.Map{}
	}

	// DO NOT catch here — propagate error
	return c.Render(view, data)
}
