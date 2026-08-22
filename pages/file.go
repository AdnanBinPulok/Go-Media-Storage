package pages

import (
	"fmt"
	"go-media-storage/classes"
	"go-media-storage/database_tables"
	"go-media-storage/settings"
	"log"

	"github.com/gofiber/fiber/v2"
)

// Get example: /file/view/abc123def456.jpg
func FilePage(router fiber.Router) {
	router.Get("/file/view/:storedName", func(c *fiber.Ctx) error {
		storedName := string(c.Params("storedName"))

		mediaFileRecord, err := database_tables.MediaFilesTableInstance.Table.FetchOne(
			"stored_name", storedName,
		)

		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("File not found")
		}

		if mediaFileRecord.Deleted {
			return c.Status(fiber.StatusNotFound).SendString("File Deleted")
		}

		filePath := mediaFileRecord.StoragePath

		OriginalName := mediaFileRecord.OriginalName
		MimeType := viewMimeType(mediaFileRecord.MimeType)
		FileSizeBytes := mediaFileRecord.FileSizeBytes

		c.Set("Content-Type", MimeType)
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Content-Disposition", "inline; filename=\""+OriginalName+"\"")
		c.Set("Content-Length", fmt.Sprint(FileSizeBytes))

		BrowserCacheTime := settings.CachingConfigInstance.BrowserCacheTime
		ProxyCacheTime := settings.CachingConfigInstance.ProxyCacheTime

		// public: anyone can cache it
		// max-age: how long the browser caches it
		// s-maxage: how long Cloudflare (shared proxies) caches it
		// immutable: the file will never change, bypasses conditional re-validation
		cacheControl := fmt.Sprintf("public, max-age=%d, s-maxage=%d, immutable", BrowserCacheTime, ProxyCacheTime)
		c.Set("Cache-Control", cacheControl)

		fileExists, err := classes.FileManagerInstance.FileExists(filePath)

		if err != nil {
			log.Printf("Error checking file existence: %v\n", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
		}
		if !fileExists {
			return c.Status(fiber.StatusNotFound).SendString("File not found")
		}

		// Resolve safe absolute path using FileManagerInstance
		fullPath, err := classes.FileManagerInstance.ResolvePath(filePath)

		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("File not found")
		}

		return c.SendFile(fullPath)
	})

	router.Get("/file/download/:storedName", func(c *fiber.Ctx) error {
		storedName := string(c.Params("storedName"))

		mediaFileRecord, err := database_tables.MediaFilesTableInstance.Table.FetchOne(
			"stored_name", storedName,
		)

		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("File not found")
		}
		if mediaFileRecord.Deleted {
			return c.Status(fiber.StatusNotFound).SendString("File Deleted")
		}

		filePath := mediaFileRecord.StoragePath

		OriginalName := mediaFileRecord.OriginalName
		MimeType := sanitizeMimeType(mediaFileRecord.MimeType)
		FileSizeBytes := mediaFileRecord.FileSizeBytes

		c.Set("Content-Type", MimeType)
		c.Set("X-Content-Type-Options", "nosniff")
		// Change "inline" to "attachment" to force download
		c.Set("Content-Disposition", "attachment; filename=\""+OriginalName+"\"")
		c.Set("Content-Length", fmt.Sprint(FileSizeBytes))

		BrowserCacheTime := settings.CachingConfigInstance.BrowserCacheTime
		ProxyCacheTime := settings.CachingConfigInstance.ProxyCacheTime

		// public: anyone can cache it
		// max-age: how long the browser caches it
		// s-maxage: how long Cloudflare (shared proxies) caches it
		// immutable: the file will never change, bypasses conditional re-validation
		cacheControl := fmt.Sprintf("public, max-age=%d, s-maxage=%d, immutable", BrowserCacheTime, ProxyCacheTime)
		c.Set("Cache-Control", cacheControl)

		fileExists, err := classes.FileManagerInstance.FileExists(filePath)

		if err != nil {
			log.Printf("Error checking file existence: %v\n", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
		}
		if !fileExists {
			return c.Status(fiber.StatusNotFound).SendString("File not found")
		}

		// Resolve safe absolute path using FileManagerInstance
		fullPath, err := classes.FileManagerInstance.ResolvePath(filePath)

		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("File not found")
		}

		return c.SendFile(fullPath)
	})
}
