package routes

import (
	"fmt"
	"go-media-storage/modules"
	"go-media-storage/services"
	"math"

	"github.com/gofiber/fiber/v2"
)

func fetchVaultRelatedStats(userID any) (map[string]any, error) {
	queryResult, err := services.DatabaseConn.RunQuery(
		"SELECT COUNT(*) AS total_vaults, COALESCE(SUM(total_files), 0)::BIGINT AS total_files, COALESCE(SUM(total_size_bytes), 0)::BIGINT AS total_size_bytes, COALESCE(SUM(storage_bytes_limit), 0)::BIGINT AS total_vaults_storage_bytes_limit FROM vaults WHERE owner_id = $1",
		[]any{userID},
	)
	if err != nil {
		return nil, err
	}
	if len(queryResult) == 0 {
		return map[string]any{
			"total_vaults":                     0,
			"total_files":                      0,
			"total_size_bytes":                 0,
			"total_vaults_storage_bytes_limit": 0,
		}, nil
	}
	return map[string]any{
		"total_vaults":                     queryResult[0]["total_vaults"],
		"total_files":                      queryResult[0]["total_files"],
		"total_size_bytes":                 queryResult[0]["total_size_bytes"],
		"total_vaults_storage_bytes_limit": queryResult[0]["total_vaults_storage_bytes_limit"],
	}, nil
}

func ratio(numerator, denominator int64) float64 {
	if denominator <= 0 {
		return 0
	}

	result := (float64(numerator) / float64(denominator)) * 100
	result = math.Round(result*100) / 100

	if result > 100 {
		return 100
	}
	if result < 0 {
		return 0
	}

	return result
}

func DashboardRoutes(router fiber.Router) {
	router.Get("/dashboard/stats", func(c *fiber.Ctx) error {
		userData := modules.FetchUserData(c)
		if userData == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Not logged in",
			})
		}

		userStorageLimitBytes := toInt64(userData["storage_bytes_limit"])
		userVaultLimit := toInt64(userData["vaults_limit"])

		statsData := map[string]any{
			// New clearer names
			"storage": map[string]any{
				"limit":                userStorageLimitBytes,
				"used":                 int64(0),   // will be updated with actual value below
				"available":            int64(0),   // will be updated with actual value below
				"used_percentage":      float64(0), // will be updated with actual value below
				"allocated":            int64(0),   // will be updated with actual value below
				"allocated_percentage": float64(0), // will be updated with actual value below
			},

			"vault": map[string]any{
				"limit":                userVaultLimit,
				"allocated":            int64(0),   // will be updated with actual value below
				"available":            int64(0),   // will be updated with actual value below
				"allocated_percentage": float64(0), // will be updated with actual value below
			},
		}

		vaultRelatedStats, err := fetchVaultRelatedStats(userData["id"])
		if err != nil {
			fmt.Printf("Error fetching vault related stats for user %d: %v\n", userData["id"], err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to fetch vault related stats",
			})
		}
		allocatedBytes := toInt64(vaultRelatedStats["total_vaults_storage_bytes_limit"])
		usedBytes := toInt64(vaultRelatedStats["total_size_bytes"])
		vaultsAllocated := toInt64(vaultRelatedStats["total_vaults"])

		statsData["storage"].(map[string]any)["used"] = usedBytes
		statsData["storage"].(map[string]any)["available"] = userStorageLimitBytes - usedBytes
		statsData["storage"].(map[string]any)["used_percentage"] = ratio(usedBytes, userStorageLimitBytes)
		statsData["storage"].(map[string]any)["allocated"] = allocatedBytes
		statsData["storage"].(map[string]any)["allocated_percentage"] = ratio(allocatedBytes, userStorageLimitBytes)

		statsData["vault"].(map[string]any)["allocated"] = vaultsAllocated
		statsData["vault"].(map[string]any)["available"] = userVaultLimit - vaultsAllocated
		statsData["vault"].(map[string]any)["allocated_percentage"] = ratio(vaultsAllocated, userVaultLimit)

		return c.JSON(fiber.Map{
			"success": true,
			"stats":   statsData,
		})
	})

	router.Get("/dashboard/file_types", func(c *fiber.Ctx) error {
		userData := modules.FetchUserData(c)
		if userData == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Not logged in",
			})
		}

		query := `
		SELECT 
			file_type AS type, 
			COALESCE(SUM(file_size_bytes), 0)::BIGINT AS total_size,
			COUNT(*)::BIGINT AS count
		FROM media_files 
		WHERE owner_id = $1 AND deleted = false
		GROUP BY file_type
		ORDER BY total_size DESC`

		queryResult, err := services.DatabaseConn.RunQuery(query, []any{userData["id"]})
		if err != nil {
			fmt.Printf("Error fetching file types for user %d: %v\n", userData["id"], err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to fetch file types data",
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"data":    queryResult,
		})
	})

}
