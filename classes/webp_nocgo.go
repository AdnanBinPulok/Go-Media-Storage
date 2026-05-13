//go:build !cgo

package classes

import (
	"image"
	"io"
)

// webpAvailable is false when CGo is disabled (no libwebp linked).
const webpAvailable = false

// encodeWebP is a no-op stub. The caller in Compressor.go checks webpAvailable
// before calling this function, so this path is never reached in practice.
func encodeWebP(_ io.Writer, _ image.Image, _ int, _ bool) error {
	return nil
}
