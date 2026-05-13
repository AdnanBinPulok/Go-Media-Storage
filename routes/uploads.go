package routes

import (
	"fmt"
	"go-media-storage/database_tables"
	"go-media-storage/modules"

	"github.com/gofiber/fiber/v2"
)

func UploadsRoutes(router fiber.Router) {
	router.Get("/uploads/recent", func(c *fiber.Ctx) error {
		userData := modules.FetchUserData(c)
		if userData == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Not logged in",
			})
		}

		whereConditions := map[string]any{
			"owner_id": userData["id"],
		}

		mediaFilePageRecords, err := database_tables.MediaFilesTableInstance.Table.FetchPages(
			1,
			18,
			"created_at",
			"desc",
			whereConditions,
		)
		if err != nil {
			fmt.Printf("Error fetching media files: %v\n", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to fetch media files",
			})
		}

		if len(mediaFilePageRecords.Items) == 0 {
			return c.JSON(fiber.Map{
				"success": true,
				"data":    []any{},
			})
		}

		safeData, err := []map[string]any{}, nil
		for _, record := range mediaFilePageRecords.Items {
			safeRecord, err := database_tables.MediaFilesTableInstance.Table.ToColumnData(record)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"message": "Failed to process media files",
				})
			}
			safeRecord["url"] = GenerateFileURL(record.StoredName)

			safeData = append(safeData, safeRecord)
		}

		return c.JSON(fiber.Map{
			"success": true,
			"data":    safeData,
		})
	})
}
