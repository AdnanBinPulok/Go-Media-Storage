//go:build cgo

package classes

import (
	"image"
	"io"

	"github.com/chai2010/webp"
)

// webpAvailable is true when the binary was compiled with CGo + libwebp.
const webpAvailable = true

// encodeWebP encodes img as WebP into w.
// quality is 1–100; lossless ignores quality and produces a lossless encode.
func encodeWebP(w io.Writer, img image.Image, quality int, lossless bool) error {
	return webp.Encode(w, img, &webp.Options{
		Lossless: lossless,
		Quality:  float32(quality),
	})
}
