package pages

import (
	"go-media-storage/modules"
	"go-media-storage/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

func DashboardPage(router fiber.Router) {
	rateLimiter := utils.RateLimiter(utils.RateLimitConfig{
		Requests: 50,
		Window:   time.Second * 5,
	})

	router.Get(
		"/dashboard",
		rateLimiter,
		func(c *fiber.Ctx) error {
			userData := modules.FetchUserData(c)
			if userData == nil {
				return c.Redirect("/login")
			}

			return utils.RenderPage(c, "pages/dashboard", fiber.Map{
				"site": modules.GetSiteData(),
				"user": userData,
			})
		},
	)

	router.Get(
		"/dashboard/vaults",
		rateLimiter,
		func(c *fiber.Ctx) error {
			userData := modules.FetchUserData(c)
			if userData == nil {
				return c.Redirect("/login")
			}

			return utils.RenderPage(c, "pages/dashboard_vaults", fiber.Map{
				"site": modules.GetSiteData(),
				"user": userData,
			})
		},
	)
	router.Get(
		"/dashboard/upload",
		rateLimiter,
		func(c *fiber.Ctx) error {
			userData := modules.FetchUserData(c)
			if userData == nil {
				return c.Redirect("/login")
			}

			return utils.RenderPage(c, "pages/dashboard_upload", fiber.Map{
				"site": modules.GetSiteData(),
				"user": userData,
			})
		},
	)

	router.Get(
		"/dashboard/compress",
		rateLimiter,
		func(c *fiber.Ctx) error {
			userData := modules.FetchUserData(c)
			if userData == nil {
				return c.Redirect("/login")
			}

			return utils.RenderPage(c, "pages/dashboard_compress", fiber.Map{
				"site": modules.GetSiteData(),
				"user": userData,
			})
		},
	)
}
