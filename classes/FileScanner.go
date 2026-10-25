package classes

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
)

// ScanResult holds the outcome of a file scan.
type ScanResult struct {
	Safe   bool
	Reason string
}

// FileScanner inspects uploaded files for dangerous content before they are stored.
type FileScanner struct{}

// FileScannerInstance is the process-wide singleton.
var FileScannerInstance = &FileScanner{}

// dangerousMimeTypes lists MIME types that a browser can execute or render as active content.
var dangerousMimeTypes = map[string]bool{
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

// dangerousExtensions lists file extensions that are executable or script-bearing.
var dangerousExtensions = map[string]bool{
	".exe":   true,
	".dll":   true,
	".so":    true,
	".bat":   true,
	".cmd":   true,
	".sh":    true,
	".ps1":   true,
	".php":   true,
	".php3":  true,
	".php4":  true,
	".php5":  true,
	".phtml": true,
	".asp":   true,
	".aspx":  true,
	".jsp":   true,
	".py":    true,
	".rb":    true,
	".pl":    true,
}

// htmlSignatures are byte sequences that indicate an HTML/script payload
// regardless of the declared MIME type or extension.
var htmlSignatures = [][]byte{
	[]byte("<!DOCTYPE"),
	[]byte("<!doctype"),
	[]byte("<html"),
	[]byte("<HTML"),
	[]byte("<script"),
	[]byte("<SCRIPT"),
}

// Scan inspects a multipart file upload and returns a ScanResult.
// It checks:
//  1. MIME type against a dangerous-types blocklist.
//  2. File extension against a dangerous-extensions blocklist.
//  3. The first 512 bytes of the file body for HTML/script signatures.
func (fs *FileScanner) Scan(file *multipart.FileHeader, detectedMime string) ScanResult {
	// 0. Path-like / percent-encoded names never reach disk.
	if err := ValidateOriginalFilename(file.Filename); err != nil {
		return ScanResult{Safe: false, Reason: err.Error()}
	}

	// 1. MIME type check
	cleanMime := strings.ToLower(strings.TrimSpace(strings.SplitN(detectedMime, ";", 2)[0]))
	if dangerousMimeTypes[cleanMime] {
		return ScanResult{
			Safe:   false,
			Reason: fmt.Sprintf("dangerous MIME type: %s", cleanMime),
		}
	}

	// 2. Extension check (basename only; already validated as non-path)
	name := strings.ToLower(filepath.Base(file.Filename))
	lastDot := strings.LastIndex(name, ".")
	if lastDot != -1 {
		ext := name[lastDot:]
		if dangerousExtensions[ext] {
			return ScanResult{
				Safe:   false,
				Reason: fmt.Sprintf("dangerous file extension: %s", ext),
			}
		}
	}

	// 3. Magic-byte / content sniff check (first 512 bytes)
	f, err := file.Open()
	if err == nil {
		defer f.Close()

		buf := make([]byte, 512)
		n, _ := io.ReadFull(f, buf)
		if n > 0 {
			head := buf[:n]
			for _, sig := range htmlSignatures {
				if bytes.Contains(head, sig) {
					return ScanResult{
						Safe:   false,
						Reason: "file content contains HTML or script signatures",
					}
				}
			}
		}
	}

	return ScanResult{Safe: true}
}
