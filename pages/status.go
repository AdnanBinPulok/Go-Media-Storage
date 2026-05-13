package pages

import (
	"go-media-storage/modules"
	"go-media-storage/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

func StatusPage(router fiber.Router) {
	rateLimiter := utils.RateLimiter(utils.RateLimitConfig{
		Requests: 50,
		Window:   time.Second * 5,
	})

	router.Get(
		"/status",
		rateLimiter,
		func(c *fiber.Ctx) error {
			userData := modules.FetchUserData(c)
			return utils.RenderPage(c, "pages/status", fiber.Map{
				"site": modules.GetSiteData(),
				"user": userData,
			})
		},
	)
}
