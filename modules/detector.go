package modules

import (
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

// DetectMimeType deeply analyzes a file to determine its MIME type.
// It relies on magic bytes using the `mimetype` library to prevent spoofing,
// and falls back to extension guessing for completely empty files.
func DetectMimeType(file *multipart.FileHeader) string {
	f, err := file.Open()
	if err == nil {
		defer f.Close()

		// The library safely reads up to 3072 bytes (the magic number limit).
		mtype, err := mimetype.DetectReader(f)
		if err == nil && mtype != nil {
			// We rewind the file pointer so that it can be read later when saving
			f.Seek(0, 0)

			// If it detects a generic bytes stream but the extension strongly implies
			// otherwise (e.g. CSVs often detect as text/plain), we could theoretically fall back,
			// but relying on the detected type is much safer against malicious uploads.
			return mtype.String()
		}
	}

	// Fallback mechanism: if opening the file fails (or it's completely empty), we rely on default OS extensions
	ext := strings.ToLower(filepath.Ext(file.Filename))
	mtype := mimetype.Lookup(ext)
	if mtype != nil {
		return mtype.String()
	}

	return "application/octet-stream"
}

// FileTypeFromMimeType groups granular MIME types into broad, front-end friendly application categories.
// It is deeply exhaustive to prevent types like 'application/x-msdownload' slipping into generic tags.
func FileTypeFromMimeType(mimeTypeStr string) string {
	mimeTypeStr = strings.ToLower(strings.TrimSpace(mimeTypeStr))
	parts := strings.SplitN(mimeTypeStr, "/", 2)

	mainType := parts[0]
	subType := ""
	if len(parts) > 1 {
		subType = strings.SplitN(parts[1], ";", 2)[0] // Clean parameters like charset=utf-8
	}

	switch mainType {
	case "image", "video", "audio", "font", "model":
		return mainType
	case "text":
		if strings.Contains(subType, "javascript") || strings.Contains(subType, "script") ||
			strings.Contains(subType, "python") || strings.Contains(subType, "html") ||
			strings.Contains(subType, "css") || strings.Contains(subType, "go") ||
			strings.Contains(subType, "c") || strings.Contains(subType, "ruby") ||
			strings.Contains(subType, "rust") || strings.Contains(subType, "java") ||
			strings.Contains(subType, "typescript") {
			return "code"
		}
		if strings.Contains(subType, "csv") || strings.Contains(subType, "xml") ||
			strings.Contains(subType, "yaml") || strings.Contains(subType, "json") {
			return "data"
		}
		if strings.Contains(subType, "markdown") || strings.Contains(subType, "rtf") {
			return "document"
		}
		return "text"
	}

	// For Application and complex subtypes, we do extremely robust string matching
	switch {
	case strings.Contains(subType, "msdownload") || strings.Contains(subType, "executable") ||
		strings.Contains(subType, "dosexec") || strings.Contains(subType, "elf") ||
		strings.Contains(subType, "mach-binary") || strings.Contains(subType, "ms-dos") ||
		strings.Contains(subType, "x-pi") || subType == "x-mz":
		return "executable"

	case (strings.Contains(subType, "zip") && !strings.Contains(subType, "epub")) ||
		strings.Contains(subType, "compressed") || strings.Contains(subType, "tar") ||
		strings.Contains(subType, "rar") || strings.Contains(subType, "gzip") ||
		strings.Contains(subType, "bzip") || strings.Contains(subType, "xz") ||
		strings.Contains(subType, "7z") || strings.Contains(subType, "apple-diskimage") ||
		strings.Contains(subType, "zstd") || strings.Contains(subType, "iso9660-image"):
		return "archive"

	case strings.Contains(subType, "pdf") || strings.Contains(subType, "document") ||
		strings.Contains(subType, "msword") || strings.Contains(subType, "excel") ||
		strings.Contains(subType, "powerpoint") || strings.Contains(subType, "rtf") ||
		strings.Contains(subType, "epub") || strings.Contains(subType, "presentation") ||
		strings.Contains(subType, "spreadsheet") || strings.Contains(subType, "pages") ||
		strings.Contains(subType, "numbers") || strings.Contains(subType, "keynote"):
		return "document"

	case strings.Contains(subType, "json") || strings.Contains(subType, "xml") ||
		strings.Contains(subType, "yaml") || strings.Contains(subType, "sql") ||
		strings.Contains(subType, "graphql") || strings.Contains(subType, "bson"):
		return "data"

	case strings.Contains(subType, "javascript") || strings.Contains(subType, "typescript") ||
		strings.Contains(subType, "python") || strings.Contains(subType, "ruby") ||
		strings.Contains(subType, "php") || strings.Contains(subType, "wasm") ||
		strings.Contains(subType, "java-source") || strings.Contains(subType, "sh"):
		return "code"
	}

	if mainType == "application" && subType == "octet-stream" {
		return "other"
	}

	if mainType == "application" {
		return "application"
	}

	return "other"
}
