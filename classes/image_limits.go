package classes

import (
	"fmt"
	"image"
	"io"
)

// Decoded-image limits. A few-KB upload can declare an enormous canvas, and the
// decoder allocates width*height before any of our code runs. Every decode path
// therefore reads the header first and refuses oversized images.
const (
	// MaxImageDimension caps one side, in pixels.
	MaxImageDimension = 20000
	// MaxImagePixels caps width*height, in pixels (~50 MP).
	MaxImagePixels = 50_000_000
)

// ValidateDecodedSize rejects canvases that would cost too much memory.
func ValidateDecodedSize(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid image dimensions %dx%d", width, height)
	}
	if width > MaxImageDimension || height > MaxImageDimension {
		return fmt.Errorf("image is too large: %dx%d (max %d per side)", width, height, MaxImageDimension)
	}
	if int64(width)*int64(height) > MaxImagePixels {
		return fmt.Errorf("image has too many pixels: %dx%d", width, height)
	}
	return nil
}

// CheckImageHeader reads only the format header and validates the declared
// canvas. It never decodes pixels, so a hostile header cannot allocate memory.
func CheckImageHeader(r io.Reader) error {
	cfg, _, err := image.DecodeConfig(r)
	if err != nil {
		return fmt.Errorf("unreadable image header: %w", err)
	}
	return ValidateDecodedSize(cfg.Width, cfg.Height)
}
