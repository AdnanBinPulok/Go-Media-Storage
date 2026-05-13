package classes

import (
	"fmt"
	"go-media-storage/database_tables"
	"go-media-storage/modules"
	"go-media-storage/services"
	"log"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"
)

type UploadManager struct {
	StoragePath string
}

var UploadManagerInstance *UploadManager

func fileExtensionFromFilename(file *multipart.FileHeader) string {
	lastDot := strings.LastIndex(file.Filename, ".")
	if lastDot == -1 || lastDot == len(file.Filename)-1 {
		return ""
	}
	return file.Filename[lastDot:]
}

func (um *UploadManager) UploadFile(file *multipart.FileHeader, vault database_tables.VaultModel, userId int64) (database_tables.MediaFileModel, error) {
	// example input will be "C:/Users/JohnDoe/Pictures/photo.jpg", and vault will be "my-vault"

	StoredName, err := GenerateRandomString(32)
	if err != nil {
		return database_tables.MediaFileModel{}, err
	}

	MimeType := modules.DetectMimeType(file)

	scanResult := FileScannerInstance.Scan(file, MimeType)
	if !scanResult.Safe {
		return database_tables.MediaFileModel{}, fmt.Errorf("file rejected: %s", scanResult.Reason)
	}

	FileExt := strings.ToLower(fileExtensionFromFilename(file))

	FileSizeBytes := file.Size

	storedFileName := StoredName + FileExt
	storagePath := filepath.ToSlash(filepath.Join(vault.VaultFolderName, storedFileName))

	now := time.Now().UTC()

	mediaRecord := database_tables.MediaFileModel{
		OwnerID:       userId,
		VaultID:       vault.ID,
		OriginalName:  file.Filename,
		MimeType:      MimeType,
		FileType:      modules.FileTypeFromMimeType(MimeType),
		FileExt:       FileExt,
		FileSizeBytes: FileSizeBytes,
		StoredName:    storedFileName,
		StoragePath:   storagePath,
		CreatedAtUTC:  now,
	}

	if FileManagerInstance == nil {
		return database_tables.MediaFileModel{}, fmt.Errorf("file manager not initialized")
	}

	fileStored, err := FileManagerInstance.WriteFile(storagePath, file)
	if err != nil || !fileStored {
		return database_tables.MediaFileModel{}, err
	}

	insertedRecord, err := database_tables.MediaFilesTableInstance.Table.InsertOne(mediaRecord)
	if err != nil {
		return database_tables.MediaFileModel{}, err
	}

	// run background task to update total media & size in vault
	go func() {
		err := um.UpdateVaultMediaStats(vault.ID)
		if err != nil {
			fmt.Printf("Error updating vault media stats for vault ID %d: %v\n", vault.ID, err)
		}
	}()

	return insertedRecord, nil
}

func (um *UploadManager) UpdateVaultMediaStats(vaultID int64) error {
	if services.DatabaseConn == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	queryResult, err := services.DatabaseConn.RunQuery(
		"SELECT COUNT(*) AS media_count, COALESCE(SUM(file_size_bytes), 0)::bigint AS total_size FROM media_files WHERE vault_id = $1 AND deleted = $2",
		[]any{vaultID, false},
	)
	if err != nil {
		log.Printf("Error counting media files for vault ID %d: %v", vaultID, err)
		return fmt.Errorf("count and sum media files: %w", err)
	}

	if len(queryResult) == 0 {
		return fmt.Errorf("count and sum media files: no result returned")
	}

	fmt.Printf("Raw Query Result %+v\n", queryResult)

	mediaCount, ok := queryResult[0]["media_count"].(int64)
	if !ok {
		return fmt.Errorf("count and sum media files: invalid media_count type")
	}
	totalSize, ok := queryResult[0]["total_size"].(int64)
	if !ok {
		return fmt.Errorf("count and sum media files: invalid total_size type")
	}

	updatedRecords, err := database_tables.VaultsTableInstance.Table.Update(
		map[string]any{
			"total_files":      mediaCount,
			"total_size_bytes": totalSize,
		},
		map[string]any{
			"id": vaultID,
		},
	)
	if err != nil {
		return fmt.Errorf("update vault media stats: %w", err)
	}
	if len(updatedRecords) == 0 {
		return fmt.Errorf("update vault media stats: no rows updated")
	}
	return nil
}

func NewUploadManager(storagePath string) *UploadManager {
	um := &UploadManager{StoragePath: storagePath}
	UploadManagerInstance = um
	return um
}
