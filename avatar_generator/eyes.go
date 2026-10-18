package avatar_generator

import (
	"image"
	"math"
)

// drawEyes places eyes relative to the face that actually got drawn, and clamps
// them so they can never sit on the body or spill past the face edge.
func drawEyes(img *image.RGBA, spec avatarSpec, size int, fi faceInfo) {
	fs := float64(size)

	w := spec.EyeW * fs
	h := spec.EyeH * fs
	gap := spec.EyeGap * fs

	if maxH := fi.radius * 0.42; h > maxH {
		h = maxH
	}
	if maxW := fi.radius * 0.55; w > maxW {
		w = maxW
	}

	// Half-span from the face center to the outer edge of an eye.
	span := gap/2 + w
	if maxSpan := fi.radius * 0.60; span > maxSpan {
		k := maxSpan / span
		gap *= k
		w *= k
		span = maxSpan
	}

	ey := fi.cy + spec.EyeDY*fs
	lx := fi.cx - span + w/2
	rx := fi.cx + span - w/2

	switch spec.Eye {
	case eyeVisor:
		hw := span
		fillSDF(img, fi.cx-hw, ey-h/2, fi.cx+hw, ey+h/2, spec.EyeCol,
			func(dx, dy float64) float64 { return roundedSDF(dx, dy, hw, h/2, h*0.45) })

	case eyeBars:
		fillRectAA(img, lx-w/2, ey-h/2, lx+w/2, ey+h/2, spec.EyeCol)
		fillRectAA(img, rx-w/2, ey-h/2, rx+w/2, ey+h/2, spec.EyeCol)

	case eyeSquares:
		s := math.Min(w, h) * 0.95
		sdf := func(dx, dy float64) float64 { return roundedSDF(dx, dy, s/2, s/2, s*0.18) }
		fillSDF(img, lx-s/2, ey-s/2, lx+s/2, ey+s/2, spec.EyeCol, sdf)
		fillSDF(img, rx-s/2, ey-s/2, rx+s/2, ey+s/2, spec.EyeCol, sdf)

	case eyeCircles:
		hx, hy := w/2, h/2
		sdf := func(dx, dy float64) float64 { return ellipseSDF(dx, dy, hx, hy) }
		fillSDF(img, lx-hx, ey-hy, lx+hx, ey+hy, spec.EyeCol, sdf)
		fillSDF(img, rx-hx, ey-hy, rx+hx, ey+hy, spec.EyeCol, sdf)

	case eyePills:
		hx, hy := w/2, h/2
		rad := math.Min(hx, hy)
		sdf := func(dx, dy float64) float64 { return roundedSDF(dx, dy, hx, hy, rad) }
		fillSDF(img, lx-hx, ey-hy, lx+hx, ey+hy, spec.EyeCol, sdf)
		fillSDF(img, rx-hx, ey-hy, rx+hx, ey+hy, spec.EyeCol, sdf)

	case eyeDiamonds:
		hx, hy := w/2, h/2
		sdf := func(dx, dy float64) float64 {
			f := math.Abs(dx)/hx + math.Abs(dy)/hy - 1
			return f / math.Hypot(1/hx, 1/hy)
		}
		fillSDF(img, lx-hx, ey-hy, lx+hx, ey+hy, spec.EyeCol, sdf)
		fillSDF(img, rx-hx, ey-hy, rx+hx, ey+hy, spec.EyeCol, sdf)
	}
}

// roundedSDF is the exact signed distance to a rounded rectangle.
func roundedSDF(dx, dy, hx, hy, rad float64) float64 {
	rad = math.Min(rad, math.Min(hx, hy))
	qx := math.Abs(dx) - (hx - rad)
	qy := math.Abs(dy) - (hy - rad)
	return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - rad
}

// ellipseSDF approximates the signed distance to an ellipse: close enough for a
// one-pixel antialiasing band, and far cheaper than the exact form.
func ellipseSDF(dx, dy, hx, hy float64) float64 {
	d := math.Sqrt((dx/hx)*(dx/hx) + (dy/hy)*(dy/hy))
	if d == 0 {
		return -math.Min(hx, hy)
	}
	// |grad| of the implicit function converts f to a distance
	g := math.Hypot(dx/(hx*hx), dy/(hy*hy)) / d
	return (d - 1) / g
}
