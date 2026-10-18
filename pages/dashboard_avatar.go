package pages

import (
	"go-media-storage/avatar_generator"
	"go-media-storage/modules"
	"go-media-storage/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

// avatarPresets are the quick-size chips. The configured default is highlighted
// at runtime, so a custom AVATAR_DEFAULT_SIZE still has a matching chip when it
// is one of these values.
var avatarPresets = []int{8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096}

// avatarTemplateData exposes the generator limits to the page instead of
// hard-coding them in HTML and JS.
func avatarTemplateData() fiber.Map {
	return fiber.Map{
		"default_size": AvatarDefaultSize(),
		"min_size":     avatar_generator.MinSize,
		"max_size":     avatar_generator.MaxSize,
		"presets":      avatarPresets,
	}
}

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
				"site":   modules.GetSiteData(),
				"user":   userData,
				"avatar": avatarTemplateData(),
			})
		},
	)
}
