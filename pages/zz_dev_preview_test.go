package pages

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
)

// TestDevPreviewServe is a throwaway harness used for a visual check of
// /dashboard/avatar without a database. It is deleted after the check.
func TestDevPreviewServe(t *testing.T) {
	engine := html.New("../static", ".html")
	if err := engine.Load(); err != nil {
		t.Fatalf("load templates: %v", err)
	}

	app := fiber.New(fiber.Config{Views: engine})
	app.Use(logger.New())

	app.Get("/dashboard/avatar", func(c *fiber.Ctx) error {
		return c.Render("pages/dashboard_avatar", fiber.Map{
			"site": map[string]any{
				"title":    "Go Media Storage",
				"favicon":  "/assets/favicon.ico",
				"base_url": "localhost:18777",
			},
			"user": map[string]any{
				"full_name": "Dev Preview",
				"email":     "dev@example.com",
			},
		})
	})

	AvatarPage(app)

	t.Log("listening on http://127.0.0.1:18777/dashboard/avatar")
	if err := app.Listen("127.0.0.1:18777"); err != nil {
		t.Fatalf("listen: %v", err)
	}
}
