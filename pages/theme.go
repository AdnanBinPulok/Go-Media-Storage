package pages

import (
	"go-media-storage/modules"
	"go-media-storage/utils"

	"github.com/gofiber/fiber/v2"
)

func ThemePage(router fiber.Router) {
	router.Get("/theme", func(c *fiber.Ctx) error {
		userData := modules.FetchUserData(c)
		return utils.RenderPage(c, "pages/theme", fiber.Map{
			"site": modules.GetSiteData(),
			"user": userData,
		})
	})
}
