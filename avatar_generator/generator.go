package avatar_generator

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"strconv"
	"sync"
)

// ErrIDTooLong is returned when the id exceeds MaxIDLength. The generator
// accepts any content, so this is the only input that can be rejected.
var ErrIDTooLong = errors.New("avatar_generator: id exceeds MaxIDLength")

// maxPooledRGBA keeps a 4096px canvas (64 MiB) from being pinned in the pool.
// Larger buffers are left to the garbage collector after a single use.
const maxPooledRGBA = 16 << 20

// rgbaPool recycles canvas buffers. Every render starts from a fully
// transparent canvas, so acquire zeroes the bytes it hands out.
var rgbaPool = sync.Pool{
	New: func() any { return &image.RGBA{} },
}

func acquireRGBA(size int) *image.RGBA {
	n := size * size * 4

	img := rgbaPool.Get().(*image.RGBA)
	if cap(img.Pix) < n {
		img.Pix = make([]byte, n)
	}
	img.Pix = img.Pix[:n]
	img.Stride = size * 4
	img.Rect = image.Rect(0, 0, size, size)
	clear(img.Pix)

	return img
}

func releaseRGBA(img *image.RGBA) {
	if cap(img.Pix) > maxPooledRGBA {
		return
	}
	rgbaPool.Put(img)
}

// GeneratePNG renders a deterministic avatar and returns PNG bytes.
//
// size is clamped to [MinSize, MaxSize] and paddingPct to [0, MaxPaddingPct].
// Same arguments always produce byte-identical output, so the result is safe to
// cache and to serve with an immutable Cache-Control header.
//
// The returned slice is freshly allocated and owned by the caller.
func GeneratePNG(id string, size int, paddingPct float64) ([]byte, error) {
	if len(id) > MaxIDLength {
		return nil, ErrIDTooLong
	}
	return renderPNG(specFromID(id), clampSize(size), clampPadding(paddingPct))
}

// ETag returns a strong cache validator for the avatar these arguments produce,
// quoted and ready for the ETag header.
//
// It is derived only from the inputs, so it stays valid across restarts and
// across instances without any stored state.
func ETag(id string, size int, paddingPct float64) string {
	h := sha256.New()
	h.Write([]byte(id))
	h.Write([]byte{0})
	h.Write([]byte(strconv.Itoa(clampSize(size))))
	h.Write([]byte{0})
	h.Write([]byte(strconv.FormatFloat(clampPadding(paddingPct), 'f', 4, 64)))

	sum := h.Sum(nil)[:8]
	return `"` + hex.EncodeToString(sum) + `"`
}

// renderPNG draws into a pooled canvas and encodes. The caller must pass
// already-clamped values.
func renderPNG(spec avatarSpec, size int, paddingPct float64) ([]byte, error) {
	img := acquireRGBA(size)
	defer releaseRGBA(img)

	drawAvatar(img, spec, size, paddingPct)
	return encodePNG(img)
}

// renderImage draws into a new canvas. Tests and benchmarks use it; the route
// path goes through renderPNG so the buffer is recycled.
func renderImage(spec avatarSpec, size int, paddingPct float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	drawAvatar(img, spec, size, paddingPct)
	return img
}

// drawAvatar is the drawing pass. Pixels are stored premultiplied (the standard
// image.RGBA layout) over a fully transparent background.
func drawAvatar(img *image.RGBA, spec avatarSpec, size int, paddingPct float64) {
	f := spec.form()
	defer f.release()

	cx := float64(size) / 2
	cy := cx
	full := img.Bounds()

	// f is fitted into [-1,1], so bodyRadius is a hard bound on the drawing.
	// An extreme padding on a tiny canvas would leave nothing to draw, so floor
	// the extent to keep at least a dot of ink.
	maxExtent := (0.5-paddingPct)*float64(size) - fringePx
	if maxExtent < 1.5 {
		maxExtent = 1.5
	}

	bodyRadius := spec.BodyScale * maxExtent

	// 1. body
	drawShape(img, f, cx, cy, bodyRadius, spec.AspectY, full,
		hslToRGB(spec.BodyHue, spec.BodySat, spec.BodyLight))

	// 2. face
	drawFace(img, spec, f, cx, cy, bodyRadius, full)

	// 3. eyes, placed relative to the face that actually got drawn
	drawEyes(img, spec, size, spec.faceInfo(cx, cy, bodyRadius))
}

func drawFace(img *image.RGBA, spec avatarSpec, f *shapeForm, cx, cy, bodyR float64, full image.Rectangle) {
	faceCol := spec.faceColor()
	fi := spec.faceInfo(cx, cy, bodyR)

	switch spec.Face {
	case faceBand:
		band := image.Rect(
			full.Min.X, int(fi.cy-fi.radius),
			full.Max.X, int(fi.cy+fi.radius),
		)
		drawShape(img, f, cx, cy, bodyR, spec.AspectY, band, faceCol)

	case faceHalf:
		half := image.Rect(
			full.Min.X, int(fi.cy-fi.radius*0.55),
			full.Max.X, full.Max.Y,
		)
		drawShape(img, f, cx, cy, bodyR, spec.AspectY, half, faceCol)

	case faceOffset:
		drawShape(img, f, fi.cx, fi.cy, fi.radius, spec.AspectY, full, faceCol)

	default: // faceFull
		drawShape(img, f, cx, cy, fi.radius, spec.AspectY, full, faceCol)
	}
}

// encodePNG writes the avatar canvas as an 8-bit RGBA PNG.
//
// The dedicated writer in pngfast.go replaces image/png, which spends most of
// its time trying five filters per row. Decoded pixels are identical; only the
// compressed bytes differ.
func encodePNG(img *image.RGBA) ([]byte, error) {
	return encodePNGFast(img)
}
