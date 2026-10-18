package main

import (
	"fmt"
	"log"

	"go-media-storage/pages"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
)

// Temporary visual harness for /dashboard/avatar. Renders the real templates
// without auth/DB so the layout can be inspected in a browser.
func main() {
	engine := html.New("./static", ".html")
	if err := engine.Load(); err != nil {
		log.Fatal(err)
	}

	app := fiber.New(fiber.Config{Views: engine})
	app.Static("/styles", "./static/styles")
	app.Static("/assets", "./static/assets")
	app.Static("/scripts", "./static/scripts")

	pages.AvatarPage(app)

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Render("pages/dashboard_avatar", fiber.Map{
			"site": map[string]any{"title": "Preview", "favicon": "/assets/favicon.ico", "base_url": "localhost"},
			"user": map[string]any{"full_name": "Preview User", "email": "preview@example.com"},
			"avatar": map[string]any{
				"default_size": 256,
				"min_size":     8,
				"max_size":     4096,
				"presets":      []int{8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096},
			},
		})
	})

	fmt.Println("preview on http://127.0.0.1:9123")
	log.Fatal(app.Listen("127.0.0.1:9123"))
}
