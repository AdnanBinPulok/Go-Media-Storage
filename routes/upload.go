package routes

import (
	"fmt"
	"go-media-storage/classes"
	"go-media-storage/database_tables"
	"go-media-storage/modules"
	"go-media-storage/settings"
	"log"

	"github.com/gofiber/fiber/v2"
)

func UploadRoutes(router fiber.Router) {
	router.Post("/upload", func(c *fiber.Ctx) error {
		vaultData := modules.FetchVaultData(c)
		if vaultData == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invalid or missing API key",
			})
		}

		form, err := c.MultipartForm()
		if err != nil || form == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "No files uploaded",
			})
		}

		filesToUpload := form.File["file"]
		if len(filesToUpload) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "No files uploaded",
			})
		}

		vaultID := vaultData["id"].(int64)

		log.Printf("Received upload request for vault_id: %d", vaultID)

		vault, err := database_tables.VaultsTableInstance.Table.FetchOne("id", vaultID)
		if err != nil {
			log.Printf("Error fetching vault with id %d: %v", vaultID, err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invalid or missing API key",
			})
		}

		// first get the current size of storage BasePath's used storage and check against the limit before writing the file
		storageUsed, err := classes.FileManagerInstance.GetStorageUsed()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Error calculating current storage usage",
			})
		}

		FileSizeBytesTotal := int64(0)
		for _, fileHeader := range filesToUpload {
			FileSizeBytesTotal += fileHeader.Size
		}

		if settings.FileStorageConfigInstance.StorageLimitBytes > 0 && (storageUsed+FileSizeBytesTotal) > settings.FileStorageConfigInstance.StorageLimitBytes {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Global Storage limit exceeded: %s used, limit is %s", FormatBytes(storageUsed), FormatBytes(settings.FileStorageConfigInstance.StorageLimitBytes)),
			})
		}

		vaultUsedStorage, err := database_tables.MediaFilesTableInstance.GetTotalStorageUsedForVault(vaultID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Error calculating vault storage usage",
			})
		}

		if vault.StorageBytesLimit > 0 && (vaultUsedStorage+FileSizeBytesTotal) > vault.StorageBytesLimit {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Vault Storage limit exceeded: %s used, limit is %s", FormatBytes(vaultUsedStorage), FormatBytes(vault.StorageBytesLimit)),
			})
		}

		uploadMediaRecords := make([]database_tables.MediaFileModel, 0, len(filesToUpload))

		for _, uploadedFile := range filesToUpload {
			log.Printf("Received file: %s", uploadedFile.Filename)
			uploaded, err := classes.UploadManagerInstance.UploadFile(uploadedFile, vault, vault.OwnerID)
			if err != nil {
				log.Printf("Error uploading file %s: %v", uploadedFile.Filename, err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"message": err.Error(),
				})
			}
			uploadMediaRecords = append(uploadMediaRecords, uploaded)
		}

		uploadMediaRecordsResponse := make([]fiber.Map, len(uploadMediaRecords))
		for i, record := range uploadMediaRecords {
			uploadMediaRecordsResponse[i] = fiber.Map{
				"id":            record.ID,
				"owner_id":      record.OwnerID,
				"vault_id":      record.VaultID,
				"stored_name":   record.StoredName,
				"original_name": record.OriginalName,
				"mime_type":     record.MimeType,
				"file_type":     record.FileType,
				"file_ext":      record.FileExt,
				"file_size":     record.FileSizeBytes,
				"created_at":    record.CreatedAtUTC,
				"url":           GenerateFileURL(record.StoredName),
			}
		}

		return c.JSON(fiber.Map{
			"success": true,
			"message": "Files uploaded successfully",
			"data":    uploadMediaRecordsResponse,
		})
	})
}
