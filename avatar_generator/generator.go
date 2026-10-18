package avatar_generator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strconv"
	"sync"
)

// ErrIDTooLong is returned when the id exceeds MaxIDLength. The generator
// accepts any content, so this is the only input that can be rejected.
var ErrIDTooLong = errors.New("avatar_generator: id exceeds MaxIDLength")

// bufPool recycles the PNG write buffer. Encode is called on every cache miss,
// so a per-call buffer would be pure garbage.
var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
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

// renderPNG draws and encodes. The caller must pass already-clamped values.
func renderPNG(spec avatarSpec, size int, paddingPct float64) ([]byte, error) {
	return encodePNG(renderImage(spec, size, paddingPct))
}

// renderImage is the drawing pass. It returns a straight-alpha RGBA image with
// a fully transparent background.
func renderImage(spec avatarSpec, size int, paddingPct float64) *image.RGBA {
	f := spec.form()
	defer f.release()

	img := image.NewRGBA(image.Rect(0, 0, size, size))
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

	return img
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

// encodePNG writes PNG at BestSpeed. BestCompression costs ~15ms more per avatar
// and saves a few KB; since responses are cached and re-sent, latency wins.
// Switch to png.DefaultCompression if bandwidth matters more than CPU.
func encodePNG(img image.Image) ([]byte, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(buf, img); err != nil {
		return nil, fmt.Errorf("avatar_generator: encode png: %w", err)
	}

	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
}
