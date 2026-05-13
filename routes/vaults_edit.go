package routes

import (
	"fmt"
	"go-media-storage/database_tables"
	"go-media-storage/modules"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// GET /api/vaults?page=1&limit=10&order_by=created_at&order=desc&filter_id=123
func VaultEditRoutes(router fiber.Router) {

	router.Post("/vaults/edit", func(c *fiber.Ctx) error {
		userData := modules.FetchUserData(c)
		if userData == nil {
			return c.Redirect("/login", fiber.StatusSeeOther)
		}

		type EditVaultRequest struct {
			ID                int64  `json:"id"`
			Name              string `json:"name"`
			IsActive          bool   `json:"is_active"`
			StorageBytesLimit int64  `json:"storage_bytes_limit"`
		}

		var body EditVaultRequest
		if err := c.BodyParser(&body); err != nil {
			return err
		}

		if body.ID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "invalid vault id",
			})
		}

		if body.Name == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "vault name is required",
			})
		}

		if body.StorageBytesLimit < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "max storage bytes cannot be negative",
			})
		}

		vault, err := database_tables.VaultsTableInstance.Table.FetchOne(map[string]any{
			"id":       body.ID,
			"owner_id": userData["id"],
		})
		if err != nil {
			log.Printf("Error fetching vault with ID %d for user %d: %v", body.ID, userData["id"], err)
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "vault not found",
			})
		}

		// checks if StorageBytesLimit + existing all vaults total_size_bytes exceeds user's storage_bytes_limit limit (if set)
		totalVaultsMaxStorageBytes, err := fetchUserVaultsTotalMaxStorageBytes(toInt64(userData["id"]))
		if err != nil {
			log.Printf("Error calculating total max storage bytes for user %d: %v", userData["id"], err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "failed to calculate storage limits",
			})
		}

		totalVaultsMaxStorageBytes = totalVaultsMaxStorageBytes - vault.StorageBytesLimit // remove current vault's max storage from total to avoid double counting

		userMaxStorageBytes := toInt64(userData["storage_bytes_limit"])
		if userMaxStorageBytes > 0 && body.StorageBytesLimit > 0 && (totalVaultsMaxStorageBytes+body.StorageBytesLimit) > userMaxStorageBytes {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Exceeds your total storage limit %s/%s.", FormatBytes(totalVaultsMaxStorageBytes+body.StorageBytesLimit), FormatBytes(userMaxStorageBytes)),
			})
		}

		_, err = database_tables.VaultsTableInstance.Table.Update(map[string]any{
			"name":                body.Name,
			"is_active":           body.IsActive,
			"storage_bytes_limit": body.StorageBytesLimit,
			"updated_at":          time.Now().UTC(),
		}, map[string]any{
			"id":       vault.ID,
			"owner_id": userData["id"],
		})
		if err != nil {
			log.Printf("Error updating vault with ID %d for user %d: %v", vault.ID, userData["id"], err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "failed to update vault",
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true,
			"message": "vault updated successfully",
		})
	})
}
