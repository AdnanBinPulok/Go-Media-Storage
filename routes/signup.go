package routes

import (
	"fmt"
	"go-media-storage/classes"
	"go-media-storage/database_tables"
	"go-media-storage/modules"
	"net/mail"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

func isSignupEmailAllowed(email string) bool {
	allowedEmails := modules.GetWhitelistedEmails()

	email = strings.ToLower(strings.TrimSpace(email))

	for _, allowed := range allowedEmails {
		if strings.ToLower(strings.TrimSpace(allowed)) == email {
			return true
		}
	}

	return false
}

func isEmailValid(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

type SignupRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func SignupRoutes(router fiber.Router) {
	router.Post("/signup", func(c *fiber.Ctx) error {
		userData := modules.FetchUserData(c)
		if userData != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "already logged in",
			})
		}

		// all in body not form data
		var body SignupRequest

		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
		}

		fullName := body.FullName
		email := body.Email
		password := body.Password
		maxVault := 1

		if fullName == "" || email == "" || password == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "full_name, email, and password are required",
			})
		}

		fullNameLength := len(fullName)
		if fullNameLength > 100 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "You Must be Kidding, " + string(rune(fullNameLength)) + "Length Full Name?",
			})
		}

		emailLength := len(email)

		if emailLength > 255 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "You Must be Kidding, " + string(rune(emailLength)) + "Length Email?",
			})
		}

		if !modules.EncryptionManagerInstance.IsPasswordStrong(password) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "password must be at least 8 characters long",
			})
		}

		if !isEmailValid(email) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "invalid email format",
			})
		}

		if !isSignupEmailAllowed(email) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "email is not allowed for signup",
			})
		}

		if database_tables.UsersTableInstance == nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "user table instance is not initialized",
			})
		}

		passwordHash, err := modules.EncryptionManagerInstance.HashPassword(password)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "failed to hash password",
			})
		}

		_, err = database_tables.UsersTableInstance.Table.FetchOne("email", strings.ToLower(strings.TrimSpace(email)))
		if err == nil {
			fmt.Println("Email already registered:", email)
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"success": false,
				"message": "email is already registered",
			})
		}

		now := time.Now().UTC()
		insertedUser, err := database_tables.UsersTableInstance.Table.InsertOne(database_tables.UserModel{
			FullName:     strings.TrimSpace(fullName),
			Email:        strings.ToLower(strings.TrimSpace(email)),
			MaxVaults:    maxVault,
			IsActive:     true,
			PasswordHash: passwordHash,
			UpdatedAtUTC: now,
			CreatedAtUTC: now,
		})
		if err != nil {
			fmt.Println("Failed to create user:", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "failed to create user",
			})
		}

		userID := insertedUser.ID

		response := fiber.Map{
			"success": true,
			"message": "user created successfully",
			"user_id": userID,
		}

		err = classes.SessionManagerInstance.AssignSession(c, userID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "user created but failed to assign session",
				"user_id": userID,
			})
		}

		return c.JSON(response)
	})
}
