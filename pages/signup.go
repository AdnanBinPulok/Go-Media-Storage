package pages

import (
	"go-media-storage/modules"
	"go-media-storage/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

func SignupPage(router fiber.Router) {
	rateLimiter := utils.RateLimiter(utils.RateLimitConfig{
		Requests: 50,
		Window:   time.Second * 5,
	})

	router.Get(
		"/signup",
		rateLimiter,
		func(c *fiber.Ctx) error {
			userData := modules.FetchUserData(c)
			if userData != nil {
				return c.Redirect("/")
			}

			return utils.RenderPage(c, "pages/signup", fiber.Map{
				"site": modules.GetSiteData(),
				"user": userData,
			})
		},
	)
}
