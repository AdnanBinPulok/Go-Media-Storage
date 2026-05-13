package utils

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func ErrorHandler(c *fiber.Ctx, err error) error {
	// Default to 500
	code := fiber.StatusInternalServerError

	// Fiber errors (404, 429, etc.)
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	log.Printf("Error %d on %s: %v", code, c.Path(), err)

	ip := c.IP()

	// 🔥 IMPORTANT: clear any partial response
	c.Response().Reset()

	// API requests → JSON
	if strings.HasPrefix(c.Path(), "/api") ||
		strings.Contains(c.Get("Accept"), "application/json") {

		return c.Status(code).JSON(fiber.Map{
			"error": err.Error(),
			"code":  code,
			"path":  c.Path(),
		})
	}

	// HTML errors
	view := "errors/500"

	switch code {
	case fiber.StatusNotFound:
		view = "errors/404"
	case fiber.StatusTooManyRequests:
		view = "errors/429"
	}

	if renderErr := c.Status(code).Render(view, fiber.Map{
		"ErrorMessage": err.Error(),
		"Code":         code,
		"Path":         c.Path(),
		"IP":           ip,
	}); renderErr != nil {
		log.Printf("Failed to render %s: %v", view, renderErr)
		return c.Status(code).SendString(err.Error())
	}

	return nil
}
