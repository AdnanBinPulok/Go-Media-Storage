package classes

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"go-media-storage/database_tables"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

type SessionManager struct {
}

func NewSessionManager() *SessionManager {
	return &SessionManager{}
}

func GenerateRandomString(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (sm *SessionManager) AssignSession(c *fiber.Ctx, userId int64) error {
	sessionToken, err := GenerateRandomString(32)
	if err != nil {
		return fmt.Errorf("generate session token: %w", err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour * 24 * 30) // 30 days
	now := time.Now().UTC()

	if database_tables.SessionsTableInstance == nil {
		return fmt.Errorf("session table instance is not initialized")
	}

	insertedSession, err := database_tables.SessionsTableInstance.Table.InsertOne(database_tables.SessionModel{
		UserID:       userId,
		SessionToken: sessionToken,
		IPAddress:    c.IP(),
		UserAgent:    c.Get("User-Agent"),
		ExpiresAtUTC: expiresAt,
		UpdatedAtUTC: now,
		CreatedAtUTC: now,
	})
	if err != nil {
		fmt.Println("Failed to create session:", err)
		return fmt.Errorf("create session: %w", err)
	}

	log.Printf("Created session with ID %d for user ID %d", insertedSession.ID, userId)

	c.Cookie(&fiber.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		HTTPOnly: true,
		SameSite: "lax",
		Secure:   true,
		Expires:  expiresAt,
	})

	_ = userId
	return nil
}

func (sm *SessionManager) InvalidateSession(sessionToken string) bool {
	return sessionToken != ""
}

var SessionManagerInstance = NewSessionManager()
