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

// maxDecodeRounds caps how many percent-encoding layers we peel off a client
// filename. Two rounds cover accidental double-encoding; anything deeper is
// refused rather than decoded forever.
const maxDecodeRounds = 2

// percentEscapePattern matches a single %XX escape.
var percentEscapePattern = regexp.MustCompile(`%[0-9a-fA-F]{2}`)

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
//
// Percent-escapes are decoded first (see NormalizeOriginalFilename), so a name
// like "Screenshot%202026-10-03%20105638.png" is accepted and stored as
// "Screenshot 2026-10-03 105638.png".
func ValidateOriginalFilename(name string) error {
	_, err := NormalizeOriginalFilename(name)
	return err
}

// NormalizeOriginalFilename decodes percent-escapes and returns the decoded name
// after validating it. Browsers and OSes often send escaped names such as
// "Screenshot%202026-10-03%20105638.png", which are benign; the dangerous ones
// only become dangerous once decoded ("%2f%2e%2e%2f" -> "/../").
//
// The returned name is safe to store as metadata and to use in a
// Content-Disposition header: no separators, no "..", no null/control bytes.
func NormalizeOriginalFilename(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("empty filename")
	}
	if strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("filename contains null byte")
	}

	decoded, err := decodePercentLayers(name)
	if err != nil {
		return "", err
	}
	if len(decoded) > 255 {
		return "", fmt.Errorf("filename too long")
	}
	if strings.ContainsRune(decoded, 0) {
		return "", fmt.Errorf("filename contains null byte")
	}
	// Control characters (newline, tab, escape) never belong in a filename and
	// are how log forging / header smuggling payloads start.
	for _, r := range decoded {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("filename contains control characters")
		}
	}

	if strings.ContainsAny(decoded, `/\:`) || strings.Contains(decoded, "..") {
		return "", fmt.Errorf("filename contains path separators")
	}

	base := filepath.Base(decoded)
	if base == "." || base == ".." || base != decoded {
		return "", fmt.Errorf("filename must be a bare name")
	}

	return decoded, nil
}

// decodePercentLayers unescapes up to maxDecodeRounds layers of percent-encoding.
//
// Malformed escapes ("100%done.jpg") are not an encoding layer, so they are left
// as literal text. When escapes survive all rounds the name is refused instead
// of being decoded forever — deep encoding is only ever used to hide a payload.
func decodePercentLayers(name string) (string, error) {
	for round := 0; round < maxDecodeRounds; round++ {
		if !percentEscapePattern.MatchString(name) {
			return name, nil
		}
		decoded, err := url.PathUnescape(name)
		if err != nil || decoded == name {
			// Malformed or no-op escape: treat the remainder as literal text.
			return name, nil
		}
		name = decoded
	}
	if percentEscapePattern.MatchString(name) {
		return "", fmt.Errorf("filename is over-encoded")
	}
	return name, nil
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
	// No trimming: a name with surrounding whitespace is not a name this
	// server wrote, and must not be normalised into one.
	if name == "" || strings.TrimSpace(name) != name || strings.ContainsRune(name, 0) {
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
