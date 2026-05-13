package modules

import (
	"fmt"
	"go-media-storage/database_tables"
	"time"

	"github.com/AdnanBinPulok/pggo"

	"github.com/gofiber/fiber/v2"
)

func FetchUserData(c *fiber.Ctx) map[string]interface{} {
	session_token := c.Cookies("session_token")
	if session_token == "" {
		fmt.Println("No session token found in cookies")
		return nil
	}

	if database_tables.SessionsTableInstance == nil || database_tables.UsersTableInstance == nil {
		fmt.Println("Table instances are not initialized")
		return nil
	}

	now := time.Now().UTC()

	sessionData, err := database_tables.SessionsTableInstance.Table.FetchOne(map[string]any{
		"session_token": session_token,
		"expires_at":    pggo.Gt(now),
	})
	if err != nil {
		fmt.Println("Invalid session token")
		return nil
	}
	userData, err := database_tables.UsersTableInstance.Table.FetchOne("id", sessionData.UserID)
	if err != nil {
		fmt.Println("Failed to fetch user by session")
		return nil
	}

	return map[string]interface{}{
		"id":                  userData.ID,
		"full_name":           userData.FullName,
		"email":               userData.Email,
		"vaults_limit":        userData.MaxVaults,
		"is_active":           userData.IsActive,
		"password_hash":       userData.PasswordHash,
		"storage_bytes_limit": userData.StorageBytesLimit,
		"updated_at":          userData.UpdatedAtUTC,
		"created_at":          userData.CreatedAtUTC,
	}
}
