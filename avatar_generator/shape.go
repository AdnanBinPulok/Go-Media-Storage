package avatar_generator

import (
	"image"
	"image/color"
	"math"
	"sync"
)

// lutSize is the angular resolution of the shape table. A power of two lets the
// interpolation index wrap with a mask instead of a branch.
const lutSize = 4096

// atanN is the resolution of the arctangent table used instead of math.Atan2.
const atanN = 4096

const halfPi = math.Pi / 2

var atanTable [atanN + 1]float64

func init() {
	for i := 0; i <= atanN; i++ {
		atanTable[i] = math.Atan(float64(i) / atanN)
	}
}

// shapeForm is a shape sampled once and fitted so its bounding box spans
// exactly [-1,1] on both axes. Because of that fit, multiplying by a radius
// bounds the drawing to a square of that half-size, which is what makes
// clipping impossible.
type shapeForm struct {
	rad       []float64 // radius per angle over [0,2pi), bbox-fitted
	ox, oy    float64   // bbox center, in fitted units
	invAspect float64
}

// formPool recycles the 32KB radius table. Draw calls are short and frequent,
// so this removes the dominant allocation from the hot path.
var formPool = sync.Pool{
	New: func() any { return &shapeForm{rad: make([]float64, lutSize)} },
}

func (f *shapeForm) release() {
	formPool.Put(f)
}

// radiusAt is the whole shape generator: a superellipse radius, optionally
// tapered toward a teardrop, plus harmonic wobble for organic asymmetry.
func (s avatarSpec) radiusAt(theta float64) float64 {
	ct, st := math.Abs(math.Cos(theta)), math.Abs(math.Sin(theta))
	r := math.Pow(math.Pow(ct, s.P)+math.Pow(st, s.P), -1.0/s.P)

	if s.Taper > 0 {
		// sin is +1 at the bottom, so this widens the base and pinches the top
		r *= 1 + s.Taper*math.Sin(theta)
	}

	r *= 1 + s.WobbleAmp*math.Sin(s.Freq1*theta+s.Phase1)
	r *= 1 + s.Wobble2Amp*math.Sin(s.Freq2*theta+s.Phase2)
	return r
}

// form samples radiusAt into a pooled LUT and rescales it so the shape's
// axis-aligned bounding box maps onto [-1,1].
//
// A superellipse with p=4 reaches ~1.09x its nominal radius at the 45 degree
// corners, and a tapered drop is not symmetric about its center at all.
// Without this fit, boxy and drop shapes leave the canvas while round ones sit
// comfortably inside it.
//
// The caller must call release exactly once when done drawing.
func (s avatarSpec) form() *shapeForm {
	f := formPool.Get().(*shapeForm)
	rad := f.rad

	step := 2 * math.Pi / float64(lutSize)
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := minX, maxX

	for i := 0; i < lutSize; i++ {
		// Index i is exactly angle i*step. Sampling and lookup must use the
		// same convention or asymmetric shapes drift off center.
		theta := float64(i) * step
		r := s.radiusAt(theta)
		rad[i] = r

		x, y := r*math.Cos(theta), r*math.Sin(theta)
		minX, maxX = math.Min(minX, x), math.Max(maxX, x)
		minY, maxY = math.Min(minY, y), math.Max(maxY, y)
	}

	// Fit against the larger axis, in canvas terms (aspect applied).
	m := math.Max((maxX-minX)/2, (maxY-minY)/2*s.AspectY)
	if m <= 0 || math.IsNaN(m) {
		m = 1
	}

	inv := 1 / m
	for i := range rad {
		rad[i] *= inv
	}

	f.ox = (minX + maxX) / 2 * inv
	f.oy = (minY + maxY) / 2 * inv
	f.invAspect = 1 / s.AspectY
	return f
}

// edgeRadius interpolates the fitted radius at direction (x,y).
// math.Atan2 is replaced by an octant-aware table lookup, which was the single
// most expensive call in the original per-pixel loop.
func (f *shapeForm) edgeRadius(x, y float64) float64 {
	ax, ay := math.Abs(x), math.Abs(y)

	var a float64
	if ax >= ay {
		if ax > 0 {
			a = atanTable[int(ay/ax*atanN)]
		}
	} else {
		a = halfPi - atanTable[int(ax/ay*atanN)]
	}

	if x < 0 {
		a = math.Pi - a
	}
	if y < 0 {
		a = -a
	}
	if a < 0 {
		a += 2 * math.Pi
	}

	pos := a * (lutSize / (2 * math.Pi))
	i := int(pos)
	frac := pos - float64(i)
	i &= lutSize - 1
	j := (i + 1) & (lutSize - 1)

	return f.rad[i] + (f.rad[j]-f.rad[i])*frac
}

// drawShape fills the fitted shape into a square of half-size radius centered
// at (cx, cy). Coverage is analytic: the distance to the edge becomes the
// alpha directly, so the boundary is antialiased in one pass with no
// supersampling and no downscale step.
func drawShape(img *image.RGBA, f *shapeForm, cx, cy, radius, aspectY float64, clip image.Rectangle, col color.RGBA) {
	if radius <= 0 {
		return
	}

	invR := 1 / radius
	invRy := invR / aspectY

	clip = clip.Intersect(img.Bounds())
	if clip.Empty() {
		return
	}

	x0 := max(clip.Min.X, int(cx-radius)-1)
	x1 := min(clip.Max.X, int(cx+radius)+1)
	y0 := max(clip.Min.Y, int(cy-radius*aspectY)-1)
	y1 := min(clip.Max.Y, int(cy+radius*aspectY)+1)

	for y := y0; y < y1; y++ {
		// canvas -> fitted shape space
		yf := (float64(y)+0.5-cy)*invRy + f.oy
		row := img.Pix[y*img.Stride:]

		for x := x0; x < x1; x++ {
			xf := (float64(x)+0.5-cx)*invR + f.ox

			d := math.Sqrt(xf*xf + yf*yf)
			a := 0.5 + (f.edgeRadius(xf, yf)-d)*radius
			if a <= 0 {
				continue
			}
			blendPX(row, x, col, a)
		}
	}
}

// fillRectAA fills an axis-aligned rectangle with correct edge coverage.
func fillRectAA(img *image.RGBA, x0, y0, x1, y1 float64, col color.RGBA) {
	r := image.Rect(int(x0)-1, int(y0)-1, int(x1)+2, int(y1)+2).Intersect(img.Bounds())

	for y := r.Min.Y; y < r.Max.Y; y++ {
		cy := axisCoverage(float64(y)+0.5, y0, y1)
		if cy <= 0 {
			continue
		}
		row := img.Pix[y*img.Stride:]
		for x := r.Min.X; x < r.Max.X; x++ {
			cx := axisCoverage(float64(x)+0.5, x0, x1)
			if cx <= 0 {
				continue
			}
			blendPX(row, x, col, cx*cy)
		}
	}
}

// axisCoverage is the fraction of a pixel covered by [lo, hi] along one axis.
func axisCoverage(center, lo, hi float64) float64 {
	c := math.Min(center+0.5, hi) - math.Max(center-0.5, lo)
	switch {
	case c <= 0:
		return 0
	case c > 1:
		return 1
	default:
		return c
	}
}

// blendPX composites an opaque color over one pixel of premultiplied RGBA.
// image.RGBA stores premultiplied values, so this is a plain lerp for all four
// channels.
func blendPX(row []byte, x int, col color.RGBA, a float64) {
	if a >= 1 {
		i := x * 4
		row[i+0], row[i+1], row[i+2], row[i+3] = col.R, col.G, col.B, 255
		return
	}

	ai := int(a*255 + 0.5)
	if ai <= 0 {
		return
	}
	inv := 255 - ai

	i := x * 4
	row[i+0] = uint8((int(col.R)*ai + int(row[i+0])*inv) / 255)
	row[i+1] = uint8((int(col.G)*ai + int(row[i+1])*inv) / 255)
	row[i+2] = uint8((int(col.B)*ai + int(row[i+2])*inv) / 255)
	row[i+3] = uint8((255*ai + int(row[i+3])*inv) / 255)
}

// fillSDF paints every pixel whose signed distance (in pixels, negative inside)
// falls inside the antialiasing band.
func fillSDF(img *image.RGBA, sx0, sy0, sx1, sy1 float64, col color.RGBA, sdf func(dx, dy float64) float64) {
	r := image.Rect(int(sx0)-1, int(sy0)-1, int(sx1)+2, int(sy1)+2).Intersect(img.Bounds())
	cx := (sx0 + sx1) / 2
	cy := (sy0 + sy1) / 2

	for y := r.Min.Y; y < r.Max.Y; y++ {
		dy := float64(y) + 0.5 - cy
		row := img.Pix[y*img.Stride:]

		for x := r.Min.X; x < r.Max.X; x++ {
			a := 0.5 - sdf(float64(x)+0.5-cx, dy)
			if a <= 0 {
				continue
			}
			blendPX(row, x, col, a)
		}
	}
}
