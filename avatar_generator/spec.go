package avatar_generator

import (
	"crypto/sha256"
	"encoding/binary"
	"image/color"
	"math"
	"math/rand/v2"
)

// Public limits and defaults.
const (
	// MinSize and MaxSize bound the output edge in pixels. Anything outside is
	// clamped, not rejected, so callers cannot get an error from a bad query
	// parameter alone.
	//
	// MaxSize is large on purpose: at 4096 the render costs roughly a second and
	// a 64 MiB transient buffer, so callers that expose the size to the public
	// must gate concurrency (see pages/avatar.go).
	MinSize = 8
	MaxSize = 4096

	// DefaultSize is the size used when the caller has no opinion.
	DefaultSize = 256

	// DefaultPaddingPct is a 5% transparent border on every side. This is what
	// keeps avatars from touching the edge of a circular crop.
	DefaultPaddingPct = 0.05

	// MaxPaddingPct is the largest accepted border; beyond this the shape
	// becomes a dot and the result is meaningless.
	MaxPaddingPct = 0.45

	// MaxIDLength caps the id used as a cache key. The generator hashes the
	// string, so any content is valid; only the length is bounded.
	MaxIDLength = 256
)

// fringePx absorbs the analytic coverage fringe (~0.5px) so the measured
// padding still clears the requested value.
const fringePx = 1.0

// shapeKind selects the silhouette family.
type shapeKind int

const (
	shapeBlob   shapeKind = iota // round, organic
	shapeBox                     // rounded square
	shapeDrop                    // teardrop: wide bottom, narrow top
	shapeLeaf                    // elongated, asymmetric
	shapePebble                  // round with a second harmonic
	shapeKindCount
)

// faceKind selects how the inner face layer covers the body.
type faceKind int

const (
	faceFull   faceKind = iota // full inner blob
	faceBand                   // horizontal stripe
	faceOffset                 // inner blob pushed to one side
	faceHalf                   // lower half
	faceKindCount
)

// eyeKind selects the eye geometry.
type eyeKind int

const (
	eyeBars     eyeKind = iota // two rectangles
	eyeSquares                 // rounded squares
	eyeCircles                 // ellipses
	eyePills                   // capsules
	eyeDiamonds                // rotated squares
	eyeVisor                   // one wide bar
	eyeKindCount
)

func (k shapeKind) String() string {
	return [...]string{"blob", "box", "drop", "leaf", "pebble"}[k]
}

func (k faceKind) String() string {
	return [...]string{"full", "band", "offset", "half"}[k]
}

func (k eyeKind) String() string {
	return [...]string{"bars", "squares", "circles", "pills", "diamonds", "visor"}[k]
}

// Variant weights. Cute leans round and simple, so round silhouettes and round
// eye shapes dominate. Every combination stays reachable, just less likely.
var (
	shapeWeights = []int{40, 5, 20, 10, 25}    // blob, box, drop, leaf, pebble
	faceWeights  = []int{45, 18, 25, 12}       // full, band, offset, half
	eyeWeights   = []int{15, 20, 30, 22, 8, 5} // bars, squares, circles, pills, diamonds, visor
)

// seedsFromID hashes the id into two independent 64-bit seeds. Hashing means
// sequential or non-UUID ids still distribute evenly.
func seedsFromID(id string) (uint64, uint64) {
	sum := sha256.Sum256([]byte(id))
	return binary.BigEndian.Uint64(sum[0:8]), binary.BigEndian.Uint64(sum[8:16])
}

// pick returns an index chosen from integer weights using exactly one RNG call,
// so the RNG stream position stays stable when weights change.
func pick(rng *rand.Rand, weights []int) int {
	total := 0
	for _, w := range weights {
		total += w
	}

	n := rng.IntN(total)
	for i, w := range weights {
		if n < w {
			return i
		}
		n -= w
	}
	return len(weights) - 1
}

// avatarSpec holds every random decision for one avatar. All randomness is
// resolved here so the drawing code is pure and reproducible.
type avatarSpec struct {
	Kind       shapeKind
	P          float64 // superellipse exponent: 2 = circle, 4 = rounded square
	Freq1      float64 // MUST be an integer
	WobbleAmp  float64
	Phase1     float64
	Freq2      float64 // MUST be an integer
	Wobble2Amp float64
	Phase2     float64
	Taper      float64 // drop only: wider bottom, pointed top
	AspectY    float64 // vertical stretch
	BodyScale  float64

	BodyHue   float64
	BodySat   float64
	BodyLight float64

	Face      faceKind
	FaceScale float64
	FaceDX    float64
	FaceDY    float64
	FaceBand  float64
	FaceHue   float64
	FaceSat   float64
	FaceLight float64

	Eye    eyeKind
	EyeW   float64 // fractions of the full canvas
	EyeH   float64
	EyeGap float64
	EyeDY  float64
	EyeCol color.RGBA
}

func specFromID(id string) avatarSpec {
	s1, s2 := seedsFromID(id)
	rng := rand.New(rand.NewPCG(s1, s2))

	// Pastel base. High lightness plus moderate saturation is what reads as
	// soft; the narrow band keeps a list of avatars visually consistent.
	bodyLight := 0.72 + rng.Float64()*0.08 // 0.72..0.80
	bodySat := 0.36 + rng.Float64()*0.20   // 0.36..0.56

	// The face is a deeper shade within the body's own hue family, so every
	// avatar is a calm two-tone instead of two clashing random colors.
	hueOffset, satAdjust := faceHueOffset(rng)
	faceHue := math.Mod(rng.Float64()*360+hueOffset+360, 360)
	faceSat := math.Max(bodySat+0.14+satAdjust, 0.30)
	faceLight := bodyLight - (0.15 + rng.Float64()*0.09)

	// Small, close-set eyes slightly below center. This is the strongest
	// "cute" lever: large wide-set eyes read as startled.
	s := avatarSpec{
		Kind:       shapeKind(pick(rng, shapeWeights)),
		P:          2.0 + rng.Float64()*2.0,
		Freq1:      2.0 + float64(rng.IntN(3)), // 2..4
		WobbleAmp:  0.015 + rng.Float64()*0.030,
		Phase1:     rng.Float64() * 2 * math.Pi,
		Freq2:      2.0 + float64(rng.IntN(3)), // 2..4
		Wobble2Amp: rng.Float64() * 0.020,
		Phase2:     rng.Float64() * 2 * math.Pi,
		Taper:      0.10 + rng.Float64()*0.12,
		AspectY:    1.0 + rng.Float64()*0.08,
		BodyScale:  0.95 + rng.Float64()*0.05,

		BodyHue:   rng.Float64() * 360,
		BodySat:   bodySat,
		BodyLight: bodyLight,

		Face:      faceKind(pick(rng, faceWeights)),
		FaceScale: 0.68 + rng.Float64()*0.08,
		FaceDX:    (rng.Float64() - 0.5) * 0.16,
		FaceDY:    (rng.Float64() - 0.5) * 0.14,
		FaceBand:  0.42 + rng.Float64()*0.12,
		FaceHue:   faceHue,
		FaceSat:   math.Min(faceSat, 0.72),
		FaceLight: faceLight,

		Eye:    eyeKind(pick(rng, eyeWeights)),
		EyeW:   0.075 + rng.Float64()*0.030,
		EyeH:   0.095 + rng.Float64()*0.040,
		EyeGap: 0.135 + rng.Float64()*0.040,
		EyeDY:  (rng.Float64() - 0.5) * 0.03,
	}
	s.normalizeKinds()
	s.EyeCol = contrastColor(s.faceColor())
	return s
}

// faceHueOffset returns the face hue shift and a saturation adjustment.
// Analogous shifts keep the pair calm; the rare wide shift is deliberately
// desaturated so it stays soft rather than clashing.
func faceHueOffset(rng *rand.Rand) (offset, satAdjust float64) {
	switch pick(rng, []int{34, 26, 20, 14, 6}) {
	case 0:
		return 0, 0
	case 1:
		return 10, -0.02
	case 2:
		return -10, -0.02
	case 3:
		return 22, -0.06
	default:
		return 168 + rng.Float64()*24, -0.20 // soft complement
	}
}

// normalizeKinds constrains a family so it looks like itself. Ranges stay
// gentle because sharp corners and deep wobble read as aggressive.
func (s *avatarSpec) normalizeKinds() {
	switch s.Kind {
	case shapeBlob:
		s.P = 2.0 + math.Min(s.P-2.0, 0.6) // 2.0..2.6
		s.Taper = 0
	case shapeBox:
		s.P = 3.2 + math.Min(s.P-2.0, 0.8) // 3.2..4.0, rounded square
		s.WobbleAmp *= 0.4
		s.Taper = 0
	case shapeDrop:
		s.P = 2.0 + math.Min(s.P-2.0, 0.6) // 2.0..2.6
		s.AspectY = 1.0 + (s.AspectY-1.0)*0.6
	case shapeLeaf:
		s.P = 2.1 + math.Min(s.P-2.0, 0.5)
		s.Wobble2Amp = 0.04 + s.Wobble2Amp
		s.Freq2 = 2
		s.Taper = 0
	case shapePebble:
		s.P = 2.1 + math.Min(s.P-2.0, 0.4)
		s.Wobble2Amp = 0.02 + s.Wobble2Amp
		s.Taper = 0
	}
}

func (s avatarSpec) faceColor() color.RGBA {
	return hslToRGB(s.FaceHue, s.FaceSat, s.FaceLight)
}

// faceInfo describes where the face layer actually landed, so eyes can be
// placed and clamped against it instead of against the body.
type faceInfo struct {
	cx, cy, radius float64
}

func (s avatarSpec) faceInfo(cx, cy, bodyR float64) faceInfo {
	switch s.Face {
	case faceBand:
		return faceInfo{cx: cx, cy: cy + s.FaceDY*bodyR*0.5, radius: s.FaceBand * bodyR}

	case faceOffset:
		limit := (1 - s.FaceScale) * bodyR * 0.75
		return faceInfo{
			cx:     cx + s.FaceDX*2*limit,
			cy:     cy + s.FaceDY*2*limit,
			radius: s.FaceScale * bodyR,
		}

	case faceHalf:
		return faceInfo{cx: cx, cy: cy + s.FaceDY*bodyR*0.4 + 0.30*bodyR, radius: 0.60 * bodyR}

	default: // faceFull
		return faceInfo{cx: cx, cy: cy, radius: s.FaceScale * bodyR}
	}
}

// clampSize and clampPadding are the single source of truth for accepted
// values. The cache key is built from their output, so equivalent requests
// share one entry.
func clampSize(size int) int {
	switch {
	case size < MinSize:
		return MinSize
	case size > MaxSize:
		return MaxSize
	default:
		return size
	}
}

func clampPadding(pct float64) float64 {
	switch {
	case math.IsNaN(pct) || pct < 0:
		return 0
	case pct > MaxPaddingPct:
		return MaxPaddingPct
	default:
		return pct
	}
}
