package modules

import (
	"fmt"
	"go-media-storage/database_tables"

	"github.com/gofiber/fiber/v2"
)

func FetchVaultData(c *fiber.Ctx) map[string]any {
	bearerToken := c.Get("Authorization")
	if bearerToken == "" {
		return nil
	}

	var token string
	fmt.Sscanf(bearerToken, "Bearer %s", &token)

	if token == "" {
		return nil
	}

	vaultData, err := database_tables.VaultsTableInstance.Table.FetchOne(
		"api_key", token,
	)
	if err != nil {
		fmt.Printf("Error validating token: %v\n", err)
		return nil
	}

	mapData, err := database_tables.VaultsTableInstance.Table.ToColumnData(vaultData)
	if err != nil {
		fmt.Printf("Error converting vault data to map: %v\n", err)
		return nil
	}

	return mapData
}
