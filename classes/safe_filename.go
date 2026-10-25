package classes

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// storedNamePattern is the only shape we ever write or serve:
// 64 hex chars from GenerateRandomString(32), optional short alphanumeric ext.
var storedNamePattern = regexp.MustCompile(`(?i)^[0-9a-f]{64}(\.[a-z0-9]{1,8})?$`)

// allowedExts is the only set of extensions written to disk. Anything else
// (including ".env", ".php", or garbage after a last-dot) is ignored in favour
// of the MIME fallback or no extension.
var allowedExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".avif": true, ".bmp": true, ".tiff": true, ".tif": true, ".ico": true,
	".mp4": true, ".webm": true, ".mov": true, ".mkv": true,
	".mp3": true, ".wav": true, ".ogg": true, ".flac": true, ".m4a": true,
	".pdf": true, ".txt": true, ".csv": true, ".json": true, ".zip": true,
	".gz": true, ".tar": true, ".7z": true, ".rar": true,
	".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
}

// mimeFallbackExt maps detected MIME types to a safe extension when the
// client-supplied name is unusable.
var mimeFallbackExt = map[string]string{
	"image/jpeg":       ".jpg",
	"image/png":        ".png",
	"image/gif":        ".gif",
	"image/webp":       ".webp",
	"image/avif":       ".avif",
	"image/bmp":        ".bmp",
	"image/tiff":       ".tiff",
	"video/mp4":        ".mp4",
	"video/webm":       ".webm",
	"audio/mpeg":       ".mp3",
	"audio/wav":        ".wav",
	"audio/ogg":        ".ogg",
	"application/pdf":  ".pdf",
	"text/plain":       ".txt",
	"text/csv":         ".csv",
	"application/zip":  ".zip",
	"application/json": ".json",
}

// ValidateOriginalFilename rejects path-like or encoded-traversal names before
// anything is written. The original name is metadata only; storage never uses it
// as a path, but we still refuse PoC names that try to look like escapes.
func ValidateOriginalFilename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("empty filename")
	}
	if len(name) > 255 {
		return fmt.Errorf("filename too long")
	}
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("filename contains null byte")
	}

	lower := strings.ToLower(name)
	if strings.Contains(lower, "%") {
		return fmt.Errorf("filename contains percent-encoding")
	}
	if strings.ContainsAny(name, `/\:`) || strings.Contains(name, "..") {
		return fmt.Errorf("filename contains path separators")
	}

	// Also catch already-decoded traversal after a client somehow decoded once.
	decoded, err := url.PathUnescape(name)
	if err == nil && decoded != name {
		if strings.ContainsAny(decoded, `/\:`) || strings.Contains(decoded, "..") {
			return fmt.Errorf("filename contains encoded path separators")
		}
	}

	base := filepath.Base(name)
	if base == "." || base == ".." || base != name {
		return fmt.Errorf("filename must be a bare name")
	}

	return nil
}

// SafeStoredExt returns a whitelist extension for on-disk storage.
// Prefer a clean client extension; otherwise derive from the detected MIME.
// Empty string means "no extension" (still a valid stored name).
func SafeStoredExt(originalName, mimeType string) string {
	base := filepath.Base(strings.TrimSpace(originalName))
	ext := strings.ToLower(filepath.Ext(base))
	if allowedExts[ext] {
		return ext
	}

	mime := strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	if fallback, ok := mimeFallbackExt[mime]; ok {
		return fallback
	}
	return ""
}

// IsSafeStoredName reports whether a view/download path parameter matches the
// names this server actually creates.
func IsSafeStoredName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsRune(name, 0) {
		return false
	}
	if strings.ContainsAny(name, `/\:`) || strings.Contains(name, "..") || strings.Contains(name, "%") {
		return false
	}
	if !storedNamePattern.MatchString(name) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext != "" && !allowedExts[ext] {
		return false
	}
	return true
}

// SanitizeDownloadFilename produces a Content-Disposition filename that cannot
// break out of the quoted-string or look like a path.
func SanitizeDownloadFilename(original, fallbackStored string) string {
	name := filepath.Base(strings.TrimSpace(original))
	name = strings.ReplaceAll(name, `"`, "")
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, `\`, "")
	name = strings.ReplaceAll(name, `/`, "")
	name = strings.TrimSpace(name)

	if name == "" || name == "." || name == ".." || strings.Contains(name, "%") {
		name = filepath.Base(fallbackStored)
	}
	if name == "" || !safeDispName(name) {
		return "download"
	}
	return name
}

func safeDispName(name string) bool {
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_' || r == ' ':
		default:
			return false
		}
	}
	return true
}
