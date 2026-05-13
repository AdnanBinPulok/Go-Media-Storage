package routes

import (
	"go-media-storage/database_tables"
	"go-media-storage/modules"
	"go-media-storage/services"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func toInt64(v any) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case float64:
		return int64(val)
	case string:
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
	}
	return 0
}

func fetchUserVaultsTotalMaxStorageBytes(userID int64) (int64, error) {
	queryResult, err := services.DatabaseConn.RunQuery(
		"SELECT COALESCE(SUM(storage_bytes_limit), 0)::BIGINT AS total_max_storage_bytes FROM vaults WHERE owner_id = $1",
		[]any{userID},
	)
	if err != nil {
		return 0, err
	}
	if len(queryResult) == 0 {
		return 0, nil
	}
	totalMaxStorageBytes := toInt64(queryResult[0]["total_max_storage_bytes"])
	return totalMaxStorageBytes, nil
}

// GET /api/vaults?page=1&limit=10&order_by=created_at&order=desc&filter_id=123
func VaultRoutes(router fiber.Router) {
	router.Get("/vaults", func(c *fiber.Ctx) error {
		userData := modules.FetchUserData(c)
		if userData == nil {
			return c.Redirect("/login", fiber.StatusSeeOther)
		}

		// page, limit in query params
		page := c.QueryInt("page", 1)
		limit := c.QueryInt("limit", 10)
		if page < 1 {
			page = 1
		}
		if limit < 1 {
			limit = 10
		}
		order_by := string(c.Query("order_by", "created_at"))
		order := string(c.Query("order", "desc"))

		whereClause := map[string]any{
			"owner_id": userData["id"],
		}

		filter_id := c.QueryInt("filter_id", 0)
		if filter_id != 0 {
			whereClause["id"] = filter_id
		}

		filter_is_active := c.Query("is_active", "")
		if filter_is_active != "" {
			if filter_is_active == "true" {
				whereClause["is_active"] = true
			} else if filter_is_active == "false" {
				whereClause["is_active"] = false
			}
		}

		vaults, err := database_tables.VaultsTableInstance.Table.FetchPages(
			page,
			limit,
			order_by,
			order,
			whereClause,
		)
		if err != nil {
			log.Printf("Error fetching vaults for user %d: %v", userData["id"], err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "failed to fetch vaults",
			})
		}

		colsData, err := database_tables.VaultsTableInstance.Table.ToColumnDataList(vaults.Items)
		if err != nil {
			log.Printf("Error converting vaults to column data for user %d: %v", userData["id"], err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "failed to process vault data",
			})
		}

		statsData := fiber.Map{
			"total":    0,
			"active":   0,
			"inactive": 0,
		}

		activeCount, err := database_tables.VaultsTableInstance.Table.Count(map[string]any{
			"owner_id":  userData["id"],
			"is_active": true,
		})
		if err != nil {
			log.Printf("Error counting active vaults for user %d: %v", userData["id"], err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "failed to fetch vault stats",
			})
		}
		inactiveCount, err := database_tables.VaultsTableInstance.Table.Count(map[string]any{
			"owner_id":  userData["id"],
			"is_active": false,
		})
		if err != nil {
			log.Printf("Error counting inactive vaults for user %d: %v", userData["id"], err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "failed to fetch vault stats",
			})
		}

		statsData["active"] = activeCount
		statsData["inactive"] = inactiveCount
		statsData["total"] = activeCount + inactiveCount

		return c.JSON(fiber.Map{
			"success": true,
			"data":    colsData,
			"stats":   statsData,
			"meta": fiber.Map{
				"page":        vaults.Page,
				"limit":       vaults.Limit,
				"total_items": vaults.Total,
				"total_pages": vaults.TotalPages,
			},
		})
	})

	router.Get("/vaults/info", func(c *fiber.Ctx) error {
		userData := modules.FetchUserData(c)
		if userData == nil {
			return c.Redirect("/login", fiber.StatusSeeOther)
		}
		vaults, err := database_tables.VaultsTableInstance.Table.FetchMany(map[string]any{
			"owner_id": userData["id"],
		})
		if err != nil {
			log.Printf("Error fetching vault info for user %d: %v", userData["id"], err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "failed to fetch vault info",
			})
		}

		//  only {id, name, is_active} for each vault
		vaultsInfo := make([]fiber.Map, len(vaults))
		for i, v := range vaults {
			vaultsInfo[i] = fiber.Map{
				"id":        v.ID,
				"name":      v.Name,
				"is_active": v.IsActive,
			}
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true,
			"data":    vaultsInfo,
		})
	})
}
