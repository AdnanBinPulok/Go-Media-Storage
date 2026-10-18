package pages

import (
	"go-media-storage/modules"
	"go-media-storage/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

// DashboardAvatarPage renders the avatar playground: pick or generate a UUID,
// choose a size, and preview the result from /avatar/:uuid.
func DashboardAvatarPage(router fiber.Router) {
	rateLimiter := utils.RateLimiter(utils.RateLimitConfig{
		Requests: 50,
		Window:   time.Second * 5,
	})

	router.Get(
		"/dashboard/avatar",
		rateLimiter,
		func(c *fiber.Ctx) error {
			userData := modules.FetchUserData(c)
			if userData == nil {
				return c.Redirect("/login")
			}

			return utils.RenderPage(c, "pages/dashboard_avatar", fiber.Map{
				"site": modules.GetSiteData(),
				"user": userData,
			})
		},
	)
}
