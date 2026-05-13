package routes

import (
	"go-media-storage/classes"
	"go-media-storage/database_tables"
	"go-media-storage/modules"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

func DeleteRoutes(router fiber.Router) {
	router.Post("/delete", func(c *fiber.Ctx) error {
		vaultData := modules.FetchVaultData(c)
		if vaultData == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invalid or missing API key",
			})
		}
		var requestData struct {
			FileUrl string `json:"url"`
		}
		if err := c.BodyParser(&requestData); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invalid request body",
			})
		}
		if requestData.FileUrl == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "url is required",
			})
		}

		stored_name := classes.ExtractStoredPathFromUrl(requestData.FileUrl)

		mediaFile, err := database_tables.MediaFilesTableInstance.Table.FetchOne(map[string]any{
			"stored_name": stored_name,
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
				"message": "File is already deleted",
			})
		}

		if classes.FileManagerInstance != nil {
			deleted, err := classes.FileManagerInstance.DeleteFile(mediaFile.StoragePath)
			if err != nil || !deleted {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"message": "Failed to delete file from storage",
				})
			}
		}

		_, err = database_tables.MediaFilesTableInstance.Table.Update(
			map[string]any{
				"deleted":    true,
				"deleted_at": time.Now().UTC(),
			},
			map[string]any{
				"id":       mediaFile.ID,
				"vault_id": vaultData["id"],
			},
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to update database record",
			})
		}

		// run background task to update total media & size in vault
		go func() {
			err := classes.UploadManagerInstance.UpdateVaultMediaStats(mediaFile.VaultID)
			if err != nil {
				log.Printf("Error updating vault media stats for vault ID %d: %v\n", mediaFile.VaultID, err)
			}
		}()

		return c.JSON(fiber.Map{
			"success": true,
			"message": "File deleted successfully",
		})
	})
}
