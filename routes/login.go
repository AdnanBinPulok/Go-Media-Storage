package routes

import (
	"fmt"
	"go-media-storage/classes"
	"go-media-storage/database_tables"
	"go-media-storage/modules"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func LoginRoutes(router fiber.Router) {
	router.Post("/login", func(c *fiber.Ctx) error {
		userData := modules.FetchUserData(c)
		if userData != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "already logged in",
			})
		}

		// all in body not form data
		var body LoginRequest

		if err := c.BodyParser(&body); err != nil {
			return err
		}

		email := body.Email
		password := body.Password

		if email == "" || password == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "email and password are required",
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

		existingUser, err := database_tables.UsersTableInstance.Table.FetchOne("email", strings.ToLower(strings.TrimSpace(email)))
		if err != nil {
			fmt.Println("Email not registered:", email)
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "email is not registered",
			})
		}

		passwordHash := existingUser.PasswordHash
		if passwordHash == "" {
			fmt.Println("Invalid password hash for user:", existingUser)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "invalid user data",
			})
		}

		if !modules.EncryptionManagerInstance.VerifyPasswordHash(passwordHash, password) {
			fmt.Println("Incorrect password for email:", email)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "incorrect password",
			})
		}

		userID := existingUser.ID

		response := fiber.Map{
			"success": true,
			"message": "user logged in successfully",
			"user_id": userID,
		}

		err = classes.SessionManagerInstance.AssignSession(c, userID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "user logged in but failed to assign session",
				"user_id": userID,
			})
		}

		return c.JSON(response)
	})
}
