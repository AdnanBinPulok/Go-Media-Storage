package routes

import (
	"go-media-storage/classes"
	"go-media-storage/database_tables"
	"go-media-storage/modules"

	"github.com/gofiber/fiber/v2"
)

func CompressRoutes(router fiber.Router) {
	router.Post("/compressor/start", func(c *fiber.Ctx) error {
		vaultData := modules.FetchVaultData(c)
		if vaultData == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invalid or missing API key",
			})
		}

		var req classes.CompressionRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invalid request body",
			})
		}

		if req.FileID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "file_id is required",
			})
		}

		if err := req.Normalize(); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": err.Error(),
			})
		}

		storedName := classes.ExtractStoredPathFromUrl(req.FileID)
		if storedName == "" {
			storedName = req.FileID
		}

		mediaFile, err := database_tables.MediaFilesTableInstance.Table.FetchOne(map[string]any{
			"stored_name": storedName,
			"vault_id":    vaultData["id"],
		})
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "File not found",
			})
		}

		if mediaFile.Deleted {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "File has been deleted",
			})
		}

		if mediaFile.FileType != "image" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Compression is only supported for image files",
			})
		}

		if mediaFile.Compressed {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "File is already compressed",
			})
		}

		go classes.RunImageCompression(mediaFile, req)

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"success": true,
			"message": "Compression started",
		})
	})

	router.Post("/frontend/compressor/start", func(c *fiber.Ctx) error {
		userData := modules.FetchUserData(c)
		if userData == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Not logged in",
			})
		}

		var req classes.CompressionRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invalid request body",
			})
		}

		if req.FileID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "file_id is required",
			})
		}

		if err := req.Normalize(); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": err.Error(),
			})
		}

		storedName := classes.ExtractStoredPathFromUrl(req.FileID)
		if storedName == "" {
			storedName = req.FileID
		}

		mediaFile, err := database_tables.MediaFilesTableInstance.Table.FetchOne(map[string]any{
			"stored_name": storedName,
			"owner_id":    userData["id"],
		})
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "File not found",
			})
		}

		if mediaFile.Deleted {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "File has been deleted",
			})
		}

		if mediaFile.FileType != "image" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Compression is only supported for image files",
			})
		}

		if mediaFile.Compressed {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "File is already compressed",
			})
		}

		go classes.RunImageCompression(mediaFile, req)

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"success": true,
			"message": "Compression started",
		})
	})
}
