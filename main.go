package main

import (
	"go-media-storage/classes"
	"go-media-storage/pages"
	"go-media-storage/routes"
	"go-media-storage/services"
	"go-media-storage/settings"
	"go-media-storage/utils"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
)

type RouteFunc func(router fiber.Router)

func RegisterPages(app *fiber.App) {
	api := app.Group("/")

	pagesList := []RouteFunc{
		pages.HomePage,
		pages.ThemePage,
		pages.StatusPage,
		pages.LoginPage,
		pages.AvatarPage,
		pages.SignupPage,
		pages.DashboardPage,
		pages.FilePage,
	}

	for _, r := range pagesList {
		r(api)
		log.Printf("Registered page: %T", r)
	}
}

func RegisterRoutes(app *fiber.App) {
	api := app.Group("/api")

	routesList := []RouteFunc{
		routes.PingRoutes,
		routes.IpRoutes,
		routes.FrontUploadRoutes,
		routes.UploadRoutes,
		routes.SignupRoutes,
		routes.LoginRoutes,
		routes.VaultRoutes,
		routes.VaultAddRoutes,
		routes.VaultEditRoutes,
		routes.VaultDeleteRoutes,
		routes.UploadsRoutes,
		routes.DeleteRoutes,
		routes.DashboardRoutes,
		routes.CompressRoutes,
		routes.StatusRoutes,
	}

	for _, r := range routesList {
		r(api)
		log.Printf("Registered route: %T", r)
	}
}

func RegisterPagesAndRoutes(app *fiber.App) {
	RegisterPages(app)
	RegisterRoutes(app)
}

func main() {
	settings.LoadEnv()

	services.SetupLogger(settings.AppInfoInstance)

	err := services.InitDatabase()
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}

	log.Printf("File storage base path: %s", settings.FileStorageConfigInstance.BasePath)

	classes.NewFileManager(settings.FileStorageConfigInstance.BasePath)

	engine := html.New("./static", ".html")
	engine.Reload(true)

	app := fiber.New(fiber.Config{
		Views:                   engine,
		ErrorHandler:            utils.ErrorHandler,
		ProxyHeader:             "X-Real-IP",
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"127.0.0.1", "::1"},
		ETag:                    true,
		BodyLimit:               1024 * 1024 * 1024, // 1 GB
	})
	app.Use(logger.New())

	app.Static("/assets", "./assets")
	app.Static("/styles", "./static/styles")

	// Register pages and API routes
	RegisterPagesAndRoutes(app)

	log.Fatal(app.Listen(settings.AppInfoInstance.ApiAddress()))
}
