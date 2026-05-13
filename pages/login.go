package pages

import (
	"go-media-storage/modules"
	"go-media-storage/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

func LoginPage(router fiber.Router) {
	rateLimiter := utils.RateLimiter(utils.RateLimitConfig{
		Requests: 50,
		Window:   time.Second * 5,
	})

	router.Get(
		"/login",
		rateLimiter,
		func(c *fiber.Ctx) error {
			userData := modules.FetchUserData(c)
			if userData != nil {
				return c.Redirect("/")
			}

			return utils.RenderPage(c, "pages/login", fiber.Map{
				"site": modules.GetSiteData(),
				"user": userData,
			})
		},
	)
}
