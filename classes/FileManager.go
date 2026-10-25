package classes

import (
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

// FileManager provides path-safe file operations scoped to a single storage root.
//
// All file operations are resolved relative to StoragePath to avoid accidental
// reads/writes outside the managed storage directory.
type FileManager struct {
	// StoragePath is the absolute root directory used for all file operations.
	StoragePath string
}

// FileManagerInstance is the process-wide singleton instance initialized by
// NewFileManager.
var FileManagerInstance *FileManager

// ResolvePath validates and resolves a user-provided relative path into an
// absolute path under StoragePath.
//
// Behavior:
// - trims whitespace from filePath
// - cleans path segments (e.g. ".", "..")
// - rejects empty and absolute paths
// - rejects any path that escapes StoragePath
//
// Returns the absolute safe path or an error if validation fails.
func (fm *FileManager) ResolvePath(filePath string) (string, error) {
	raw := strings.TrimSpace(filePath)
	if raw == "" || strings.ContainsRune(raw, 0) {
		return "", fmt.Errorf("invalid file path")
	}
	// Percent-encoded separators must never appear in stored paths.
	if strings.Contains(raw, "%") {
		return "", fmt.Errorf("invalid file path")
	}
	// Storage paths are generated ASCII only. Rejecting every non-ASCII and
	// control rune stops look-alike separators (U+FF0F, U+FF3C) and full-width
	// dots from ever being treated as a filename here.
	for _, r := range raw {
		if r < 0x20 || r > 0x7e {
			return "", fmt.Errorf("invalid file path")
		}
	}

	cleanRel := filepath.Clean(raw)
	if cleanRel == "" || cleanRel == "." {
		return "", fmt.Errorf("invalid file path")
	}
	if filepath.IsAbs(cleanRel) || strings.HasPrefix(cleanRel, "/") || strings.HasPrefix(cleanRel, `\`) {
		// On Windows filepath.IsAbs("/etc/passwd") is false; still reject slash roots.
		return "", fmt.Errorf("absolute file path is not allowed")
	}
	// After Clean, reject any remaining parent references.
	if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("file path escapes storage root")
	}

	baseAbs, err := filepath.Abs(fm.StoragePath)
	if err != nil {
		return "", fmt.Errorf("resolve storage path: %w", err)
	}

	targetAbs, err := filepath.Abs(filepath.Join(baseAbs, cleanRel))
	if err != nil {
		return "", fmt.Errorf("resolve file path: %w", err)
	}

	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return "", fmt.Errorf("validate file path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("file path escapes storage root")
	}

	return targetAbs, nil
}

// WriteFile writes data to a file under StoragePath using an atomic replace
// strategy (temporary file + rename).
//
// Steps:
// - validate and resolve target path
// - ensure parent directories exist
// - write content to a temporary file in the same directory
// - apply file mode 0644
// - rename temp file to target path
//
// Returns true on success; otherwise false with a wrapped error.
func (fm *FileManager) WriteFile(filePath string, file *multipart.FileHeader) (bool, error) {
	fullPath, err := fm.ResolvePath(filePath)
	if err != nil {
		return false, fmt.Errorf("write file: %w", err)
	}

	dirPath := filepath.Dir(fullPath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return false, fmt.Errorf("write file: create parent directory: %w", err)
	}

	tempFile, err := os.CreateTemp(dirPath, ".tmp-*")
	if err != nil {
		return false, fmt.Errorf("write file: create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	fileHandle, err := file.Open()
	if err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return false, fmt.Errorf("write file: open uploaded file: %w", err)
	}
	defer fileHandle.Close()

	fileBytes, err := io.ReadAll(fileHandle)
	if err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return false, fmt.Errorf("write file: read uploaded file: %w", err)
	}

	if _, err := tempFile.Write(fileBytes); err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return false, fmt.Errorf("write file: write temp content: %w", err)
	}

	if err := tempFile.Chmod(0644); err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return false, fmt.Errorf("write file: set permissions: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return false, fmt.Errorf("write file: close temp file: %w", err)
	}

	err = os.Rename(tempPath, fullPath)
	if err != nil {
		_ = os.Remove(tempPath)
		return false, fmt.Errorf("write file: %w", err)
	}
	return true, nil
}

// DeleteFile removes a file under StoragePath.
//
// If the file is already missing, this function returns success (true, nil)
// to keep delete behavior idempotent.
func (fm *FileManager) DeleteFile(storedPath string) (bool, error) {
	fullPath, err := fm.ResolvePath(storedPath)
	if err != nil {
		return false, fmt.Errorf("delete file: %w", err)
	}

	err = os.Remove(fullPath)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("delete file: %w", err)
	}
	return true, nil
}

// FileExists reports whether a file exists under StoragePath.
//
// Returns:
// - (true, nil) if the file exists
// - (false, nil) if the file does not exist
// - (false, err) for validation/stat errors
func (fm *FileManager) FileExists(storedPath string) (bool, error) {
	fullPath, err := fm.ResolvePath(storedPath)
	if err != nil {
		return false, fmt.Errorf("check file exists: %w", err)
	}

	_, err = os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check file exists: %w", err)
	}
	return true, nil
}

// ensureStoragePath normalizes StoragePath to an absolute path and creates the
// storage directory if it does not exist.
//
// This is intended to be called during manager initialization.
func (fm *FileManager) ensureStoragePath() error {
	absPath, err := filepath.Abs(fm.StoragePath)
	if err != nil {
		return fmt.Errorf("resolve storage directory path: %w", err)
	}
	fm.StoragePath = absPath

	if _, err := os.Stat(fm.StoragePath); os.IsNotExist(err) {
		err := os.MkdirAll(fm.StoragePath, 0755)
		if err != nil {
			return fmt.Errorf("create storage directory: %w", err)
		}
	}
	return nil
}

// GetStoragePath returns the normalized absolute storage root path.
func (fm *FileManager) GetStoragePath() string {
	return fm.StoragePath
}

// GetFileMetadata retrieves os.FileInfo for a file under StoragePath.
//
// It validates the input path before calling os.Stat.
func (fm *FileManager) GetFileMetadata(storedPath string) (os.FileInfo, error) {
	fullPath, err := fm.ResolvePath(storedPath)
	if err != nil {
		return nil, fmt.Errorf("get file metadata: %w", err)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("get file metadata: %w", err)
	}
	return info, nil
}

func (fm *FileManager) GetStorageUsed() (int64, error) {
	var totalSize int64 = 0
	err := filepath.WalkDir(fm.StoragePath, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				totalSize += info.Size()
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("calculate storage used: %w", err)
	}
	return totalSize, nil
}

// NewFileManager constructs and initializes a FileManager for storagePath.
//
// On success it also sets FileManagerInstance to the newly created manager.
// Panics if the storage path cannot be validated/created.
func NewFileManager(storagePath string) *FileManager {
	fm := &FileManager{StoragePath: storagePath}
	if err := fm.ensureStoragePath(); err != nil {
		panic(fmt.Sprintf("failed to ensure storage path: %v", err))
	}
	FileManagerInstance = fm
	return fm
}

func ExtractStoredPathFromUrl(fileUrl string) string {
	parts := strings.Split(fileUrl, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
