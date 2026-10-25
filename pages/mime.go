package pages

import (
	"mime"
	"strings"
)

// dangerousMimes are MIME types a browser can execute as a script or render as
// a live document. Matched on the bare media type, so parameters such as
// "text/html; charset=utf-8" cannot slip past the check.
var dangerousMimes = map[string]bool{
	"text/html":                 true,
	"text/javascript":           true,
	"application/javascript":    true,
	"application/xhtml+xml":     true,
	"text/xml":                  true,
	"application/xml":           true,
	"image/svg+xml":             true,
	"application/x-httpd-php":   true,
	"text/x-php":                true,
	"application/x-sh":          true,
	"application/x-shellscript": true,
}

// bareMime returns a lowercase media type with all parameters stripped.
func bareMime(value string) string {
	value = strings.TrimSpace(value)
	if parsed, _, err := mime.ParseMediaType(value); err == nil {
		return strings.ToLower(parsed)
	}
	if idx := strings.IndexByte(value, ';'); idx >= 0 {
		value = value[:idx]
	}
	return strings.ToLower(strings.TrimSpace(value))
}

// sanitizeMimeType returns application/octet-stream for any MIME type that a
// browser could execute as a script or render as a webpage, preventing stored XSS.
func sanitizeMimeType(mimeType string) string {
	if dangerousMimes[bareMime(mimeType)] {
		return "application/octet-stream"
	}
	return mimeType
}

// viewMimeType adjusts MIME types so browsers can inline-play files on /file/view.
func viewMimeType(mimeType string) string {
	mimeType = sanitizeMimeType(mimeType)
	if bareMime(mimeType) == "video/quicktime" {
		return "video/mp4"
	}
	return mimeType
}
