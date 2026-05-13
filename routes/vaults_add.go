package routes

import (
	"fmt"
	"go-media-storage/classes"
	"go-media-storage/database_tables"
	"go-media-storage/modules"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// GET /api/vaults?page=1&limit=10&order_by=created_at&order=desc&filter_id=123
func VaultAddRoutes(router fiber.Router) {
	router.Post("/vaults/add", func(c *fiber.Ctx) error {
		userData := modules.FetchUserData(c)
		if userData == nil {
			return c.Redirect("/login", fiber.StatusSeeOther)
		}

		type AddVaultRequest struct {
			Name              string `json:"name"`
			StorageBytesLimit int64  `json:"storage_bytes_limit"`
		}
		// all in body not form data
		var body AddVaultRequest

		if err := c.BodyParser(&body); err != nil {
			return err
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

		totalVaults, err := database_tables.VaultsTableInstance.Table.Count(map[string]any{
			"owner_id": userData["id"],
		})
		if err != nil {
			log.Printf("Error counting vaults for user %d: %v", userData["id"], err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "failed to count existing vaults",
			})
		}

		vaults_limit := toInt64(userData["vaults_limit"])

		if totalVaults >= vaults_limit {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("%d/%d vault limit reached. Please delete existing vaults to create new ones.", totalVaults, vaults_limit),
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

		userMaxStorageBytes := toInt64(userData["storage_bytes_limit"])
		log.Printf("User %d has total vaults max storage bytes: %d", userData["id"], totalVaultsMaxStorageBytes)
		log.Printf("User %d is trying to allocate max storage bytes: %d to new vault", userData["id"], body.StorageBytesLimit)
		log.Printf("User %d has max storage bytes limit: %d", userData["id"], userMaxStorageBytes)

		if (userMaxStorageBytes > 0 && body.StorageBytesLimit > 0) && ((totalVaultsMaxStorageBytes + body.StorageBytesLimit) > userMaxStorageBytes) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Exceeds your total storage limit %s/%s.", FormatBytes(totalVaultsMaxStorageBytes+body.StorageBytesLimit), FormatBytes(userMaxStorageBytes)),
			})
		}

		api_key, err := classes.GenerateRandomString(32)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "failed to generate API key",
			})
		}

		now := time.Now().UTC()

		VaultFolderName, err := classes.GenerateRandomString(12)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "failed to generate vault folder name",
			})
		}

		newVault := database_tables.VaultModel{
			OwnerID:           toInt64(userData["id"]),
			Name:              body.Name,
			APIKey:            api_key,
			IsActive:          true,
			VaultFolderName:   VaultFolderName,
			TotalFiles:        0,
			TotalSizeBytes:    0,
			StorageBytesLimit: body.StorageBytesLimit,
			CreatedAtUTC:      now,
		}
		_, err = database_tables.VaultsTableInstance.Table.InsertOne(newVault)
		if err != nil {
			log.Printf("Error creating vault for user %d: %v", userData["id"], err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "failed to create vault",
			})
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true,
			"message": "vault created successfully",
		})
	})
}
