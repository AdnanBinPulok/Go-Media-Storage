package routes

import (
	"errors"

	"go-media-storage/classes"

	"github.com/gofiber/fiber/v2"
)

// uploadErrorResponse answers a failed upload. Rejections caused by the file
// itself (bad name, dangerous content) are client errors, so they return 400
// instead of blaming the server with a 500.
func uploadErrorResponse(c *fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	if errors.Is(err, classes.ErrFileRejected) {
		status = fiber.StatusBadRequest
	}

	return c.Status(status).JSON(fiber.Map{
		"success": false,
		"message": err.Error(),
	})
}
