# Avatar Generation — Step by Step

Deterministic PNG avatars from any string (UUID), using only the standard
library plus `golang.org/x/image`.

The implementation lives in the `avatar_generator` package. The HTTP route is
intentionally not included here; section 15 shows the whole handler.

The contract:

```go
// Same arguments in -> identical PNG bytes out, forever.
func GeneratePNG(id string, size int, paddingPct float64) ([]byte, error)
```

| arg | meaning |
|---|---|
| `id` | any string up to 256 bytes; hashed to a seed, so `user_42` works as well as a UUID |
| `size` | output edge in pixels, clamped to 8..4096, default `AVATAR_DEFAULT_SIZE` (256) |
| `paddingPct` | minimum transparent border per side, as a fraction of width (`0.05` = 5%), clamped to 0..0.45 |

Budget: **~4.8 ms** per 256 px avatar, **~14 ms** at 512 px, single goroutine.
At the 4096 maximum a render costs roughly a second and a 64 MiB buffer, so the
route throttles those separately.
Cache hits cost **~235 ns**. Both are measured, not estimated — see section 10.

---

## 0. Layout

```
avatar_generator/
├── doc.go            package documentation
├── spec.go           seeds, spec, variants, weights, faceInfo, clamps
├── shape.go          shape math, LUT, analytic AA, blending, parallel rows
├── eyes.go           6 eye styles
├── color.go          HSL + contrast
├── generator.go      GeneratePNG, canvas pool, drawAvatar, drawFace, ETag
├── pngfast.go        fixed-filter PNG writer (zlib level 1)
├── parallel.go       row-band fan-out for the drawing and filter passes
├── cache.go          TTL + LRU cache with singleflight
├── golden_test.go    decoded-pixel digests, env-gated update
├── bench_test.go     GeneratePNG 256..2048, draw-only, encode-only
├── encode_bench_test.go  filter pass and deflate levels
├── generator_test.go determinism, padding, variants, ETag
├── cache_test.go     hit/miss, TTL, LRU, byte cap, concurrency
├── flight_test.go    leader election, coalesced bursts, error cleanup
└── preview_test.go   contact sheet, env-gated
```

```powershell
go test ./avatar_generator                 # ~1.5s
go test ./avatar_generator -bench . -benchtime 50x
```

Visual review without a separate `main` package:

```powershell
$env:AVATAR_PREVIEW_DIR="$env:TEMP\avatar_preview"
go test ./avatar_generator -run TestWritePreviews
start "$env:TEMP\avatar_preview\index.html"
```

Writes 25 sample ids, every one of the 120 shape/face/eye combinations, and
padding demos to a single page.

---

## 1. The one idea: randomness as a pure function

```
id  -> sha256 -> two uint64 -> rand.New(PCG) -> avatarSpec -> pixels -> PNG
```

```go
func seedsFromID(id string) (uint64, uint64) {
	sum := sha256.Sum256([]byte(id))
	return binary.BigEndian.Uint64(sum[0:8]), binary.BigEndian.Uint64(sum[8:16])
}
```

Two rules the rest depends on:

1. **Never** use package-level `rand.Float64()`. It is auto-seeded randomly and
   destroys determinism.
2. **Never** call `time.Now()` inside the generator.

Hashing (rather than parsing the UUID) means any string is a valid id, and
sequential ids (`v1`, `v2`, DB counters) do not produce a predictable sweep.

---

## 2. All randomness goes into one spec

Decide every random value up front, then draw from the struct. Never call the
RNG inside a drawing function.

```go
type avatarSpec struct {
	Kind       shapeKind
	P          float64 // superellipse exponent: 2 = circle, 6 = boxy
	Freq1      float64 // MUST be an integer
	WobbleAmp  float64
	Phase1     float64
	Freq2      float64 // MUST be an integer
	Wobble2Amp float64
	Phase2     float64
	Taper      float64 // drop only
	AspectY    float64 // vertical stretch
	BodyScale  float64

	BodyHue, BodySat, BodyLight float64

	Face      faceKind
	FaceScale float64
	FaceDX    float64
	FaceDY    float64
	FaceBand  float64
	FaceHue, FaceSat, FaceLight float64

	Eye    eyeKind
	EyeW, EyeH, EyeGap, EyeDY float64
	EyeCol color.RGBA
}
```

Three habits that keep output from looking muddy:

- **Chain the draws.** `bodyLight` first, then `faceLight` derived *from* it,
  then `EyeCol` *from* the face color. Independent uniforms produce
  near-identical pairs; chained draws produce palettes.
- **Clamp with minimum deltas.** `faceLight = bodyLight - (0.15 + rand*0.09)`.
  Two lightnesses 0.02 apart read as a rendering bug.
- **One unit for everything.** All eye/face fractions are of the full canvas,
  never of the shape, so nothing explodes at small sizes.

---

## 3. Shape math

A circle is `x² + y² = 1`. Generalize the exponent:

\[
\left|\frac{x}{a}\right|^p + \left|\frac{y}{b}\right|^p = 1
\quad\Longrightarrow\quad
r(\theta) = \left(|\cos\theta|^p + |\sin\theta|^p\right)^{-1/p}
\]

| `p` | look |
|---|---|
| 2 | circle |
| 3 | soft blob |
| 4–5 | rounded square |
| 6+ | boxy |

Break the symmetry with **radial harmonics** — that is what reads as organic
rather than geometric:

```go
func (s avatarSpec) radiusAt(theta float64) float64 {
	ct, st := math.Abs(math.Cos(theta)), math.Abs(math.Sin(theta))
	r := math.Pow(math.Pow(ct, s.P)+math.Pow(st, s.P), -1.0/s.P)

	if s.Taper > 0 { // +sin is 1 at the bottom: wide base, pointed top
		r *= 1 + s.Taper*math.Sin(theta)
	}

	r *= 1 + s.WobbleAmp*math.Sin(s.Freq1*theta+s.Phase1)
	r *= 1 + s.Wobble2Amp*math.Sin(s.Freq2*theta+s.Phase2)
	return r
}
```

### The seam bug

`theta` wraps at `±π`, so `sin(f·θ)` is continuous across the seam only when
`f` is an **integer**:

\[
\sin(f(\theta + 2\pi)) = \sin(f\theta) \iff f \in \mathbb{Z}
\]

A random float frequency leaves a visible notch at 180°. Randomize the **phase**
and keep the frequency integral: `2 + rng.IntN(4)`.

---

## 4. Fitting: why nothing clips

A superellipse has no constant radius. Measure it:

| `p` | radius at 45° |
|---|---|
| 2 | 1.000 × nominal |
| 4 | 1.090 × nominal |
| 6 | 1.260 × nominal |

Combine with `AspectY` (up to 1.18) and a boxy shape reaches **~1.5× the radius
you thought you drew**. Set that radius to 96% of the half-canvas and the
corners leave the image — round avatars look fine, boxy ones get cut off.

So sample the shape once, measure its **bounding box**, and rescale so the box
maps onto `[-1, 1]`:

```go
func (s avatarSpec) form() *shapeForm {
	step := 2 * math.Pi / float64(lutSize)
	rad := make([]float64, lutSize)
	xs := make([]float64, lutSize)
	ys := make([]float64, lutSize)

	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := minX, maxX

	for i := 0; i < lutSize; i++ {
		// index i is exactly angle i*step (see the phase trap below)
		theta := float64(i) * step
		r := s.radiusAt(theta)
		rad[i] = r
		xs[i] = r * math.Cos(theta)
		ys[i] = r * math.Sin(theta)

		minX, maxX = math.Min(minX, xs[i]), math.Max(maxX, xs[i])
		minY, maxY = math.Min(minY, ys[i]), math.Max(maxY, ys[i])
	}

	// fit against the larger axis, in canvas terms (aspect applied)
	m := math.Max((maxX-minX)/2, (maxY-minY)/2*s.AspectY)
	inv := 1 / m
	for i := range rad {
		rad[i] *= inv
	}

	return &shapeForm{
		rad:       rad,
		ox:        (minX + maxX) / 2 * inv, // bbox center, for recentering
		oy:        (minY + maxY) / 2 * inv,
		invAspect: 1 / s.AspectY,
	}
}
```

`ox, oy` matter: a tapered drop is **not symmetric about its center**, so its
bbox center is not the origin. Shifting by `ox, oy` when sampling is what puts
the *bbox* in the middle of the canvas instead of the shape's centroid.

### The LUT phase trap (the bug that only hits asymmetric shapes)

The first version sampled `theta = -π + (i+0.5)*step` but looked up the table
with `index = angle/step`. That is a **half-bin and 180° offset**: every shape
was drawn with its radius sampled from the opposite side.

Symmetric shapes hid it — a point-reflected symmetric shape has the same bbox.
Tapered drops did not: the reflected shape's bbox center is `-oy`, so
recentering by `+oy` pushed it `2·oy` off center and it clipped. One broken
row in the debug output named it instantly:

```
CLIP kilo-23  sz=32  kind=drop  P=2.80  taper=0.34  oy=0.315
```

Fix: index `i` is angle `i*step`, no offset, interpolation between
`rad[i]` and `rad[(i+1) & (lutSize-1)]`.

---

## 5. Rasterizing with analytic coverage

No supersampling, no downscale pass. For each pixel compute the distance to the
edge and turn it into coverage directly:

```go
a := 0.5 + (edgeRadius(xf, yf) - dist) * radius
```

`a` is 1 well inside, 0 well outside, and crosses a one-pixel band at the edge.
That is mathematically better antialiasing than 4× downsampling and about 20×
cheaper.

```go
func drawShape(img *image.RGBA, f *shapeForm, cx, cy, radius, aspectY float64, clip image.Rectangle, col color.RGBA) {
	invR := 1 / radius
	invRy := invR / aspectY

	clip = clip.Intersect(img.Bounds())
	x0 := max(clip.Min.X, int(cx-radius)-1)
	x1 := min(clip.Max.X, int(cx+radius)+1)
	y0 := max(clip.Min.Y, int(cy-radius*aspectY)-1)
	y1 := min(clip.Max.Y, int(cy+radius*aspectY)+1)

	for y := y0; y < y1; y++ {
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
```

Four details:

1. **`+0.5`** samples pixel centers; without it the shape sits half a pixel off.
2. **Restrict to the bbox** — the scan is ~78% of the canvas for the body and a
   fraction of that for the face.
3. **`a = 0.5 + (r - d)*radius`** converts a distance in normalized units into
   an approximate pixel coverage (multiplied by `radius` px).
4. **`clip`** lets the band/half faces reuse the same body shape, cut to a
   rectangle.

### No Atan2 per pixel

`math.Atan2` is the single most expensive call in the loop. Replace it with an
octant-aware arctangent **table**:

```go
var atanTable [atanN + 1]float64

func init() {
	for i := 0; i <= atanN; i++ {
		atanTable[i] = math.Atan(float64(i) / atanN)
	}
}

func (f *shapeForm) edgeRadius(x, y float64) float64 {
	ax, ay := math.Abs(x), math.Abs(y)

	var a float64
	if ax >= ay {
		if ax > 0 {
			a = atanTable[int(ay/ax*atanN)]
		}
	} else {
		a = math.Pi/2 - atanTable[int(ax/ay*atanN)]
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
```

`lutSize` is a power of two, so wrapping is a mask, not a branch.

> `math.Sqrt` per pixel is fine — it is a single hardware instruction. The
> expensive ones were `Atan2`, `Pow`, and `Sin`.

---

## 6. Blending (premultiplied alpha)

`image.RGBA` stores **premultiplied** values, which is exactly why blending an
opaque color over any existing pixel is a 3-term lerp:

```go
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
```

Write through `img.Pix[y*img.Stride:]` directly. `SetRGBA` goes through an
interface and re-checks bounds on every call — it was a measurable share of the
old runtime.

Because alpha starts at 0 and we only ever composite opaque colors, the
background stays fully transparent with no cleanup pass.

---

## 6.5 Aesthetic: neutral but cute

The first version generated "correct" avatars that nobody would want as a
profile picture. The fixes were all in the spec, not the renderer.

### Problem 1 — hue was fully independent

Body and face each got `rand()*360`. That produces pairs like magenta + lime,
which is technically valid and visually hostile. Cute/neutral means **the face
is a deeper shade of the body's own hue**:

```go
func faceHueOffset(rng *rand.Rand) (off, satAdj float64) {
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
		// soft complement, pulled back toward neutral
		return 168 + rng.Float64()*24, -0.20
	}
}
```

80% analogous, 20% complement that is deliberately **desaturated**. Every
avatar is a two-tone of one family, so it stays calm, and avatars still differ
from each other because the base hue still spans the wheel.

### Problem 2 — saturation and lightness were too far apart

```go
bodyLight := 0.72 + rng.Float64()*0.08 // 0.72..0.80
bodySat   := 0.36 + rng.Float64()*0.20 // 0.36..0.56
```

Old values were `0.40..0.58` lightness at `0.55..0.80` saturation — mid-tone and
loud. Pastel lives at high lightness **and** moderate saturation. The band is
narrow on purpose: wide bands mean some UUIDs come out washed out and others
suddenly punchy, so a list of avatars looks inconsistent.

The face takes `bodyLight - (0.15..0.24)`, so it always separates from the body
without needing a second random color.

### Problem 3 — eyes were too big and too far apart

This is the single biggest "cute" lever. Large, wide-set eyes read as
**startled**; small, close-set eyes read as **gentle**.

```go
EyeW:   0.075 + rng.Float64()*0.030, // was 0.10..0.16
EyeH:   0.095 + rng.Float64()*0.040, // was 0.16..0.26
EyeGap: 0.135 + rng.Float64()*0.040, // was 0.24..0.34
EyeDY:  (rng.Float64() - 0.5) * 0.03,
```

Roughly 40% smaller and 45% closer together. Height stays slightly greater than
width, which is what makes a shape read as an "eye" rather than a "block".

### Problem 4 — eyes were pure black

```go
var (
	eyeDark  = color.RGBA{R: 58, G: 54, B: 64, A: 255}   // #3A3640 warm charcoal
	eyeLight = color.RGBA{R: 255, G: 252, B: 247, A: 255} // #FFFCF7 warm white
)
```

`#000` on a pastel face looks like a hole. A warm charcoal keeps the contrast
ratio while softening the read. The threshold moved from `0.40` to `0.34`
because pastel faces are brighter, so `0.40` was selecting white eyes too often.

### Problem 5 — silhouettes were too aggressive

`p` up to 6 (sharp box), wobble up to 0.12, `AspectY` up to 1.18, and
`leaf` added `+0.10` to the second harmonic. Together that produced lumpy,
angular shapes. Current ranges: `p` maxes at 4 (a *rounded* square), wobble
maxes at 0.045, aspect at 1.08, and the leaf boost dropped to `+0.04`.

### Problem 6 — uniform variant odds

Some variants are cuter than others, so they should be more likely. Weighted
picks, one `rng` call each:

```go
var (
	shapeWeights = []int{40, 5, 20, 10, 25}    // blob, box, drop, leaf, pebble
	faceWeights  = []int{45, 18, 25, 12}       // full, band, offset, half
	eyeWeights   = []int{15, 20, 30, 22, 8, 5} // bars, squares, circles, pills, diamonds, visor
)

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
```

Blob and pebble dominate; box is 5%. Circles and pills dominate eyes; visor is
5% because it is the least cute. All 120 combinations remain reachable, so the
gallery still shows every one.

---

## 7. Variants

Three independent axes, all seeded from the same UUID:

```go
const (
	shapeBlob shapeKind = iota // round, organic
	shapeBox                   // rounded square
	shapeDrop                  // teardrop, wide bottom
	shapeLeaf                  // elongated, asymmetric
	shapePebble                // two harmonics, wavy
	shapeKindCount
)

const (
	faceFull   faceKind = iota // inner blob
	faceBand                   // horizontal stripe
	faceOffset                 // inner blob pushed to one side
	faceHalf                   // lower half
	faceKindCount
)

const (
	eyeBars eyeKind = iota // two rectangles
	eyeSquares             // rounded squares
	eyeCircles             // ellipses
	eyePills               // capsules
	eyeDiamonds            // rotated squares
	eyeVisor               // one wide bar
	eyeKindCount
)
```

Select with `pick(rng, shapeWeights)` and friends, then **constrain** so each
family looks like itself:

```go
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
		s.P = 2.0 + math.Min(s.P-2.0, 0.6)
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
```

Without this, a "box" with `p = 2` is just a circle and the variety collapses.

### Eyes follow the face

If eyes are placed at the canvas center they break on offset and half faces. So
the face layer describes itself first:

```go
type faceInfo struct{ cx, cy, radius float64 }
```

...and eyes are drawn and clamped **against that**, not against the body:

```go
if maxH := fi.radius * 0.42; h > maxH { h = maxH }
span := gap/2 + w
if maxSpan := fi.radius * 0.60; span > maxSpan {
	k := maxSpan / span
	gap *= k
	w *= k
}
```

Shapes are drawn from signed distance functions with an AA band, so every eye
style has clean edges without special cases:

```go
// exact signed distance to a rounded rectangle
func roundedSDF(dx, dy, hx, hy, rad float64) float64 {
	rad = math.Min(rad, math.Min(hx, hy))
	qx := math.Abs(dx) - (hx - rad)
	qy := math.Abs(dy) - (hy - rad)
	return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - rad
}
```

---

## 8. Color and contrast

HSL is the right space here: vary hue freely, keep lightness controlled.

```go
func hslToRGB(h, s, l float64) color.RGBA   // standard conversion, see color.go
```

Eye color is chosen from **measured luminance**, not HSL lightness — pure yellow
and pure blue both have `L = 0.5` but read completely differently:

```go
func relativeLuminance(c color.RGBA) float64 // WCAG, with gamma expansion

var (
	eyeDark  = color.RGBA{R: 58, G: 54, B: 64, A: 255}   // #3A3640 warm charcoal
	eyeLight = color.RGBA{R: 255, G: 252, B: 247, A: 255} // #FFFCF7 warm white
)

func contrastColor(bg color.RGBA) color.RGBA {
	if relativeLuminance(bg) > 0.34 {
		return eyeDark
	}
	return eyeLight
}
```

The threshold is the one knob: lower it for more white eyes, raise it for more
charcoal. `0.34` suits a pastel palette; `0.40` was selecting white eyes too
often against bright faces.

---

## 9. Assembly, padding, encoding

```go
func GeneratePNG(id string, size int, paddingPct float64) ([]byte, error) {
	if len(id) > MaxIDLength {
		return nil, ErrIDTooLong
	}
	return renderPNG(specFromID(id), clampSize(size), clampPadding(paddingPct))
}

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

	drawShape(img, f, cx, cy, bodyRadius, spec.AspectY, full,
		hslToRGB(spec.BodyHue, spec.BodySat, spec.BodyLight))

	drawFace(img, spec, f, cx, cy, bodyRadius, full)

	drawEyes(img, spec, size, spec.faceInfo(cx, cy, bodyRadius))

	return img
}
```

Two constants worth understanding:

```go
const DefaultPaddingPct = 0.05 // caller passes this when it has no opinion
const DefaultSize       = 256  // fallback when AVATAR_DEFAULT_SIZE is unset
const fringePx = 1.0           // AAA fringe half-width, in final pixels
```

`fringePx` exists because the coverage band is about half a pixel wide on each
side of the geometric edge. Without subtracting it, a requested 5% measures as
4.5% and a "0% padding" avatar shaves ink off the border pixels.

The radius floor (`maxExtent >= 1.5`) only matters at `MinSize`: at 8 px with a
large padding the computed extent goes negative and the avatar would be blank.

**Padding is a floor, not an exact value.** `BodyScale` in `[0.95, 1.0]` adds up
to 2.5% extra size variety, so measured padding is `requested + 0..2.5%`.

### Encoding

`pngfast.go` writes the PNG directly instead of going through `image/png`.

`image/png` tries all five filter types on every row and then deflates whichever
wins; at 1024 px that costs more than drawing the avatar. Avatars are flat,
smooth shapes over a transparent background, so a fixed **Up** filter compresses
well without any per-row trial, and `klauspost/compress/zlib` at level 1
deflates faster than `compress/flate`.

The premultiplied → straight-alpha conversion copies `image/png`'s own shortcut
byte for byte (including its rounding), so switching writers cannot change a
pixel. `TestPixelGolden` locks a SHA-256 of the decoded pixels for a fixed
id/size matrix and would fail on any drift.

```go
const m = 0x101 * 0xffff // 8-bit -> 16-bit, then divide out alpha
dst[0] = uint8(uint32(src[0]) * m / (uint32(a) * 0x101) >> 8)
```

Encoding is split into two parallel stages:

1. `filterRows`: premultiply-convert + Up-filter, one goroutine per row band.
2. `compress`: a single zlib stream over the filtered scanlines.

The tradeoff is payload size: a 1024 px avatar is ~46 KB instead of ~31 KB with
the stdlib's adaptive filter choice, because a single fixed filter cannot be
optimal on every row. That is a one-time cost per cache entry, paid to halve
cold-render latency.

---

## 10. Measured results

From the package benchmarks, Intel i5-10400 (6 cores / 12 threads), Windows:

```
BenchmarkGeneratePNG256-12     2.2 ms/op    0.3 MB/op    75 allocs/op
BenchmarkGeneratePNG512-12     3.7 ms/op    0.5 MB/op    84 allocs/op
BenchmarkGeneratePNG1024-12   10.0 ms/op    0.4 MB/op    90 allocs/op
BenchmarkGeneratePNG2048-12   31.9 ms/op    1.6 MB/op   125 allocs/op
BenchmarkDrawOnly1024-12       4.0 ms/op
BenchmarkEncodeOnly1024-12     6.8 ms/op
BenchmarkFilterRows1024-12     1.1 ms/op
BenchmarkCacheHit-12          213 ns/op
```

The same code before this optimization pass:

```
BenchmarkGeneratePNG256-12     5.0 ms/op   1.13 MB/op
BenchmarkGeneratePNG512-12    14.4 ms/op   1.94 MB/op
BenchmarkGeneratePNG1024-12   55.0 ms/op   5.20 MB/op   (draw 25 / encode 33)
BenchmarkDrawOnly1024-12      25.0 ms/op
BenchmarkEncodeOnly1024-12    33.0 ms/op
```

That is roughly 5.5× at the 1024 px size and 4.5× at 2048 px. Note that most of
the win is wall-clock parallelism, so a 2-vCPU host lands closer to 3-4×.
Cost still scales with area, so the maximum is the outlier: a 4096 px avatar
allocates a 64 MiB canvas plus 67 MB of scanlines and takes a few hundred
milliseconds of CPU. That is why `pages/avatar.go` throttles renders above
1024 px through a two-slot gate instead of letting them run unbounded.

Where the time went, and what each step replaced:

| Step | Original | Now |
|---|---|---|
| shape sampling | `Atan2` + `Pow` + `Sin` per pixel | LUT + atan table + `Sqrt` |
| antialiasing | 4× canvas + CatmullRom downscale | analytic coverage, one pass |
| interior pixels | full edge lookup per pixel | solid fast path (squared-distance bound) |
| exterior pixels | full edge lookup, then reject | squared-distance reject before `Sqrt` |
| drawing | one goroutine | row bands, up to 8 goroutines |
| PNG encode | 5 filters/row + `BestSpeed` | fixed Up filter + zlib level 1 |
| RGBA buffer | allocated per render | pooled, cleared, recycled |
| **total (1024)** | **~55 ms** | **~10 ms** |


Progress along the way: 400 ms (naive per-pixel trig) → 75 ms (LUT + 4× downscale
+ BestCompression) → 4.8 ms (analytic coverage) → 2.2 ms at 256 px / 10 ms at
1024 px (parallel bands + fixed-filter PNG).

---

## 11. Verification checklist

`go test ./avatar_generator` enforces all of these, so a regression fails the
build instead of needing a visual check:

| Check | Test | Expected |
|---|---|---|
| deterministic | `TestDeterministic` | same args twice, byte-identical |
| min padding | `TestPaddingRespected` | `>= requested`, at 64/128/256/512 px |
| clipped images | `TestPaddingRespected` | no ink in row/column 0 or `size-1` |
| clamping | `TestClamping` | out-of-range args equal the documented min/max |
| transparent background | `TestOutputShape` | corner alpha `0` |
| size | `TestOutputShape` | `(size,size)` |
| variety | `TestVariantCoverage` | all 5 shapes, 4 faces, 6 eyes reachable |
| id limits | `TestIDTooLong` | `ErrIDTooLong` above 256 bytes |
| ETag | `TestETag` | stable, quoted, varies with all three inputs |
| cache | `cache_test.go` | hit/miss, TTL, LRU, byte cap, concurrency |
| coalescing | `flight_test.go` | one leader per key, result shared, key released on error |
| pixels unchanged | `TestPixelGolden` | SHA-256 of decoded pixels per id/size |
| avg time | `BenchmarkGeneratePNG256` | under 3 ms at 256 px |
| avg time | `BenchmarkGeneratePNG1024` | under 15 ms at 1024 px |

```go
// inkBounds decodes the PNG and finds the ink bbox, ignoring alpha <= 16 so
// the AA fringe is not counted as ink.
minPad, touchesEdge := inkBounds(t, pngBytes, size)
```

The HTTP layer has its own suite in `pages/avatar_test.go`:

| Check | Test | Expected |
|---|---|---|
| status and headers | `TestAvatarRouteHeadersAndCaching` | 200, `image/png`, TTL headers, quoted ETag, MISS then HIT |
| UUID equivalence | `TestAvatarRouteNormalizesUUID` | every spelling returns identical bytes |
| UUID rejection | `TestAvatarRouteRejectsInvalidUUID` | 400 for non-UUIDs |
| size rejection | `TestAvatarRouteRejectsInvalidSize` | 400 for `?size=abc` |
| size handling | `TestAvatarRouteSizes` | default 256, min 8, max 4096 |
| configurable default | `TestAvatarDefaultSizeFromEnv` | `AVATAR_DEFAULT_SIZE` reaches route + dashboard, clamps out-of-range |
| conditional requests | `TestAvatarRouteNotModified` | 304 with an empty body |
| ETag keying | `TestAvatarRouteETagVariesWithSize` | ETag changes with size |
| dashboard templates | `TestDashboardAvatarTemplateRenders` | page executes and contains the controls |

---

## 12. Tuning knobs

Aesthetic first — these are the ones that change how it *feels*:

| Knob | Effect |
|---|---|
| `bodyLight 0.72..0.80` | whole-palette brightness; go higher for airier, lower for richer |
| `bodySat 0.36..0.56` | pastel vs lively; above 0.65 stops reading as soft |
| `faceLight` offset `-0.15..-0.24` | body/face separation; too small looks like a rendering bug |
| `faceHueOffset` weights | how often the face is analogous vs soft-complement |
| `EyeW 0.075..0.105` | eye width; bigger reads cuter up to a point, then startled |
| `EyeGap 0.135..0.175` | closeness; the strongest cuteness lever |
| `EyeH` vs `EyeW` | taller-than-wide reads as an eye, wider reads as a block |
| `eyeDark` / `eyeLight` | warm charcoal instead of `#000` keeps it gentle |
| `contrastColor` threshold `0.34` | lower = more white eyes, higher = more charcoal |
| `*Weights` tables | which variants dominate; all combos stay reachable |

Renderer and layout:

| Knob | Effect |
|---|---|
| `P` ranges in `normalizeKinds` | how round vs boxy each family gets |
| `WobbleAmp 0.015..0.045` | organic bend; above ~0.08 gets lumpy |
| `Freq1` 2..4 | 2 = one big bend, 4 = wavy. **Integer only.** |
| `AspectY 1.0..1.08` | vertical stretch; above 1.15 starts looking like an egg |
| `BodyScale 0.95..1.0` | optical size variety; 1.0 = exactly the padded box |
| `FaceScale 0.68..0.76` | how much of the body is face |
| `lutSize 4096` | angular resolution of the shape table |
| `fringePx` | raise if measured padding falls short of requested |
| `DefaultPaddingPct` | default border, 5% |

---

## 13. Pitfalls

1. **Global RNG** — `rand.Float64()` from the package is randomly seeded.
   Always `rand.New(rand.NewPCG(s1, s2))`.
2. **Any clock read in the generator** — breaks determinism across restarts and
   replicas.
3. **Non-integer wobble frequency** — notch at `theta = ±π` (section 3).
4. **LUT phase mismatch** — sampling angle and lookup angle must use the same
   convention, or asymmetric shapes drift off center (section 4).
5. **Assuming a constant radius** — superellipse corners reach 1.26×; always fit
   the bbox.
6. **Drawing at 4× and encoding there** — 16× the bytes for no visual gain.
7. **Placing eyes at the canvas center** — breaks on offset/half faces.
8. **Skipping UUID normalization** — the generator hashes any string, so
   `550E...` and `550e...` would be different avatars and two cache entries. The
   route normalizes first; keep that step if you add another entry point.
9. **Look versioning** — changing `P` or the palette changes every existing
   avatar. If that matters later, hash a version prefix: `sha256("v2:" + id)`.
10. **Binding `max-age` to the in-memory TTL** — the browser copy is independent
    of the server cache. A long `max-age` means `Purge()` cannot reach clients
    that already have the image, which is fine for a 1-day avatar TTL and wrong
    for anything that changes.

---

## 14. The cache

`cache.go` holds a TTL + LRU cache built from `container/list` and a map, with no
background goroutine and no third-party dependency.

```go
type CacheConfig struct {
	TTL      time.Duration // how long a result stays valid
	MaxItems int           // hard count cap
	MaxBytes int64         // hard memory cap, 0 = unlimited
}

func DefaultCacheConfig() CacheConfig // 5 min, 10 000 items, 256 MiB
```

The route overrides the TTL from `AVATAR_MEMORY_TTL_SECONDS` (default 1 hour).
Avatars never change, so the TTL only trades memory for redraws; the LRU caps
are what bound memory.

Four decisions worth calling out:

**The key is every input that changes pixels.**

```go
func cacheKey(id string, size int, paddingPct float64) string // id \x00 size \x00 padding@4dp
```

`size` and `paddingPct` must be in the key. Keying on the id alone means the
first caller to ask for a 64 px avatar poisons 256 px requests.

Padding is stored at fixed precision *after* clamping, so `0.05`, `0.0500001`
and `1.0` (clamped to `0.45`) collapse onto the correct entry instead of
creating near-duplicates.

**Generation happens outside the lock, once per key.**

```go
if png, ok := c.get(key); ok {
	return png, true, nil
}
if f, leader := c.beginFlight(key); !leader {
	<-f.done
	return f.png, false, f.err
}
png, err := renderPNG(specFromID(id), size, paddingPct) // no lock held
c.put(key, png)
c.endFlight(key, png, err)
```

Holding a mutex while drawing would serialize the whole server behind one
avatar. Instead a small in-package singleflight coalesces concurrent misses for
the same key: the leader renders, waiters block on a channel, and the result is
in the cache before the channel closes. A failed render still releases the key.


**Both limits are enforced.** `evict` walks from the LRU tail until the item
count *and* byte total fit:

```go
for c.order.Len() > 0 && (c.order.Len() > c.maxItems ||
	(c.maxBytes > 0 && c.bytes > c.maxBytes)) {
	c.remove(c.order.Back())
}
```

The byte cap matters because item count says nothing about memory: a 256 px PNG
is ~5–10 KB, a 512 px one is roughly 4× that.

Expiry is lazy, checked on read. That avoids a janitor goroutine; `MaxItems`
already bounds memory, so stale entries cannot pile up.

`Stats()` returns `Items`, `Bytes`, `Hits`, `Misses`, `Evictions`. Everything is
mutex-protected, and a `nil *Cache` is valid — it just generates without
caching, which keeps call sites free of nil checks.

`Purge()` exists for one specific case: changing the look. Every previously
generated avatar becomes stale at that moment.

---

## 15. Wiring the route

The generator knows nothing about HTTP. Everything below is caller policy, and
it lives in `pages/avatar.go` plus `pages/avatar_store.go`.

### The handler

```go
func AvatarPage(router fiber.Router) {
	rateLimiter := utils.RateLimiter(utils.RateLimitConfig{
		Requests: 300,
		Window:   time.Second * 10,
		Scope:    "avatar", // keep this counter out of the dashboard limiter
	})

	router.Get("/avatar/:uuid", rateLimiter, func(c *fiber.Ctx) error {
		// Every valid UUID spelling normalizes to one lowercase canonical form,
		// so it maps to one avatar and one cache entry.
		id, ok := utils.NormalizeUUID(c.Params("uuid"))
		if !ok {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid UUID")
		}

		size := avatarDefaultSize() // AVATAR_DEFAULT_SIZE, clamped to 8..4096
		if raw := c.Query("size"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).SendString("Invalid size")
			}
			size = clampAvatarSize(parsed)
		}

		// Oversized renders are gated. Queue time is not generation time, so it
		// is spent outside the measurement below.
		if size > heavySize {
			if !acquireHeavySlot() {
				return fiber.NewError(fiber.StatusServiceUnavailable, "Too many large avatar requests")
			}
			defer releaseHeavySlot()
		}

		genStart := time.Now()
		png, hit, err := avatarStore().PNG(id, size, avatar_generator.DefaultPaddingPct)
		genTime := time.Since(genStart)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Failed to generate avatar")
		}

		setAvatarCacheHeaders(c, id, size) // Cache-Control, CDN headers, ETag

		if c.Fresh() { // If-None-Match matched: the client already has these bytes
			c.Status(fiber.StatusNotModified)
			return nil
		}

		c.Set(fiber.HeaderContentType, "image/png")
		c.Set("X-Content-Type-Options", "nosniff")
		if hit {
			c.Set("X-Avatar-Cache", "HIT")
		} else {
			c.Set("X-Avatar-Cache", "MISS")
		}
		// Debug only: cache hits report a few microseconds. Cloudflare caches
		// this header too, so an edge hit still shows the origin's value.
		c.Set("X-Avatar-Generation-Time", fmt.Sprintf("%dus", genTime.Microseconds()))

		return c.Send(png)
	})
}
```

### Cache headers

```go
func setAvatarCacheHeaders(c *fiber.Ctx, id string, size int) {
	ttl := settings.CachingConfigInstance.AvatarCacheTime // AVATAR_CACHE_TIME_SECONDS, default 86400
	if ttl <= 0 {
		ttl = 86400
	}

	c.Set(fiber.HeaderCacheControl, fmt.Sprintf("public, max-age=%d, s-maxage=%d, immutable", ttl, ttl))
	c.Set("CDN-Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", ttl))
	c.Set("Cloudflare-CDN-Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", ttl))
	c.Set(fiber.HeaderETag, avatar_generator.ETag(id, size, avatar_generator.DefaultPaddingPct))
}
```

Precedence at Cloudflare is `Cloudflare-CDN-Cache-Control` >
`CDN-Cache-Control` > `s-maxage` > `max-age`, so the edge TTL can be raised
without touching what browsers do.

### UUID validation

`utils.NormalizeUUID` accepts canonical, uppercase, 32-hex, braced, `urn:uuid:`
and percent-encoded forms, then returns the lowercase canonical spelling. Two
consequences worth knowing:

- `550E...`, `550e...`, the 32-hex form and `urn:uuid:550e...` are the same
  avatar and the same cache entry.
- Anything that is not 32 hex characters is rejected with `400` before it can
  become a cache key, so `user_42` no longer works. Hash any non-UUID identifier
  before putting it in the URL if you need that.

### Cloudflare

The origin headers alone are not enough: `/avatar/<uuid>` has no file extension,
so Cloudflare's default static cache ignores it. Add a **Cache Rule**:

| Setting | Value |
|---|---|
| Expression | `(http.request.uri.path matches "^/avatar/")` |
| Cache eligibility | Eligible for cache |
| Edge TTL | Use cache-control header if present |
| Cache key | Standard (keep the query string) |

Keeping the cache level **Standard** is required: `?size=256` and `?size=512` are
different images, so ignoring the query string would serve the wrong dimensions.

### Oversized renders

`avatarPNG` gates renders above 1024 px through a 2-slot channel:

```go
if size > heavySize {
	if !acquireHeavySlot() { // waits up to 5s
		return nil, false, fiber.NewError(fiber.StatusServiceUnavailable, "Too many large avatar requests")
	}
	defer releaseHeavySlot()
}
```

At 4096 each render allocates a 64 MiB RGBA buffer, so unbounded concurrency is
the only real memory hazard in this feature. Cache hits still pass through the
gate, which is intentional — they release it in microseconds.

### Rules the handler must follow

1. **Never mutate the returned slice.** It is shared with the cache.
2. **Do not lower the browser TTL below the edge TTL for no reason.** Here the
   browser copy (1 day) deliberately outlives the server copy (5 min cache), so
   calling `Purge()` after a look change cannot reach clients that already hold
   the image. That is the trade-off of a long `max-age`.
3. **Use the ETag.** Output is deterministic, so `ETag(id, size, padding)` is a
   strong validator that survives restarts and works across replicas without
   shared state.
4. **Bound the id length** (`MaxIDLength`, 256 bytes). Normalization already
   caps a UUID at 36 bytes; this protects the generator when called directly.

The property the whole design rests on:

```go
func GeneratePNG(id string, size int, paddingPct float64) ([]byte, error)
```

No globals read, no mutation, no clock. Same arguments in, same bytes out.

---

## 16. The dashboard playground

`/dashboard/avatar` is a thin client over the route: no new endpoint, no server
state. It exists to make the generator explorable.

| File | Role |
|---|---|
| `pages/dashboard_avatar.go` | login-gated page handler |
| `static/pages/dashboard_avatar.html` | shell (header + sidebar) |
| `.../dashboard_avatar/main.html` | composes the four sections and the script |
| `.../props/controls.html` | UUID input, size input, preset chips, buttons |
| `.../props/preview.html` | `<img>`, request URL, metadata, download |
| `.../props/gallery.html` | 12 random tiles at 128 px |
| `.../props/script.html` | randomUUID, normalize, preview, gallery logic |
| `.../props/styles.html` | transparency checkerboard |

Behavior worth noting:

- Empty UUID + Generate produces a random v4 UUID, so the page never shows an
  error for the most obvious action.
- The client-side `normalizeUUID` mirrors the Go version, so the page rejects the
  same inputs the server would and shows the canonical spelling it will request.
- Size is clamped to 8..4096 with a warning notification rather than an error.
- Gallery tiles use `?size=128`; each one is a distinct cache entry, which is
  why the route's rate limit is set to 300 per 10 seconds.

