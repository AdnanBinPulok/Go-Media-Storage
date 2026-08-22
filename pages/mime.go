package pages

import "strings"

// sanitizeMimeType returns application/octet-stream for any MIME type that a
// browser could execute as a script or render as a webpage, preventing stored XSS.
func sanitizeMimeType(mime string) string {
	switch mime {
	case "text/html", "text/javascript", "application/javascript",
		"application/xhtml+xml", "text/xml", "application/xml",
		"image/svg+xml", "application/x-httpd-php", "text/x-php":
		return "application/octet-stream"
	}
	return mime
}

// viewMimeType adjusts MIME types so browsers can inline-play files on /file/view.
func viewMimeType(mime string) string {
	mime = sanitizeMimeType(mime)
	if strings.EqualFold(mime, "video/quicktime") {
		return "video/mp4"
	}
	return mime
}
