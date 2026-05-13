package pages

import (
	"fmt"
	"go-media-storage/classes"
	"go-media-storage/modules"
	"go-media-storage/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

func HomePage(router fiber.Router) {

	rateLimiter := utils.RateLimiter(utils.RateLimitConfig{
		Requests: 50,
		Window:   time.Second * 5,
	})

	router.Get(
		"/",
		rateLimiter,
		func(c *fiber.Ctx) error {
			userData := modules.FetchUserData(c)

			traceId, err := classes.GenerateRandomString(16)
			if err != nil {
				return fmt.Errorf("generate trace ID: %w", err)
			}

			render := utils.RenderPage(c, "pages/home", fiber.Map{
				"site": modules.GetSiteData(),
				"user": userData,
			})

			c.Set("X-Trace-ID", traceId)
			return render
		},
	)
}
