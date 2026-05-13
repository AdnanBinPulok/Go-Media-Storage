package routes

import (
	"go-media-storage/database_tables"
	"go-media-storage/modules"
	"log"

	"github.com/gofiber/fiber/v2"
)

// GET /api/vaults?page=1&limit=10&order_by=created_at&order=desc&filter_id=123
func VaultDeleteRoutes(router fiber.Router) {
	router.Post("/vaults/delete", func(c *fiber.Ctx) error {
		userData := modules.FetchUserData(c)
		if userData == nil {
			return c.Redirect("/login", fiber.StatusSeeOther)
		}
		type DeleteVaultRequest struct {
			ID int64 `json:"id"`
		}
		var body DeleteVaultRequest
		if err := c.BodyParser(&body); err != nil {
			return err
		}

		vault, err := database_tables.VaultsTableInstance.Table.FetchOne(map[string]any{
			"id":       body.ID,
			"owner_id": userData["id"],
		})
		if err != nil {
			log.Printf("Error fetching vault with ID %d for user %d: %v", body.ID, userData["id"], err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Vault not found",
			})
		}

		_, err = database_tables.VaultsTableInstance.Table.Delete(map[string]any{
			"id":       vault.ID,
			"owner_id": userData["id"],
		})
		if err != nil {
			log.Printf("Error deleting vault with ID %d for user %d: %v", vault.ID, userData["id"], err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "failed to delete vault",
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true,
			"message": "vault deleted successfully",
		})
	})

}
