package avatar_generator

import (
	"bytes"
	"errors"
	"image/png"
	"testing"
)

var testIDs = []string{
	"550e8400-e29b-41d4-a716-446655440000",
	"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
	"9f1c2e3d-4a5b-6c7d-8e9f-0a1b2c3d4e5f",
	"user_42",
	"team-alpha",
	"alpha", "bravo", "charlie", "delta", "echo",
}

func TestDeterministic(t *testing.T) {
	for _, id := range testIDs {
		a, err := GeneratePNG(id, 256, DefaultPaddingPct)
		if err != nil {
			t.Fatalf("GeneratePNG(%q): %v", id, err)
		}
		b, err := GeneratePNG(id, 256, DefaultPaddingPct)
		if err != nil {
			t.Fatalf("GeneratePNG(%q) second call: %v", id, err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("id %q produced different bytes on repeated calls", id)
		}
	}
}

func TestDifferentIDsDiffer(t *testing.T) {
	seen := make(map[string]string, len(testIDs))
	for _, id := range testIDs {
		data, err := GeneratePNG(id, 128, DefaultPaddingPct)
		if err != nil {
			t.Fatalf("GeneratePNG(%q): %v", id, err)
		}
		key := string(data)
		if prev, ok := seen[key]; ok {
			t.Errorf("ids %q and %q produced identical output", prev, id)
		}
		seen[key] = id
	}
}

func TestOutputShape(t *testing.T) {
	// MaxSize is excluded here: a 4096 render is valid but slow, and the size
	// math is identical at every scale.
	for _, size := range []int{MinSize, 64, DefaultSize, 512} {
		data, err := GeneratePNG("shape-check", size, DefaultPaddingPct)
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}

		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("size %d: decode: %v", size, err)
		}

		if got := img.Bounds().Dx(); got != size {
			t.Errorf("width = %d, want %d", got, size)
		}
		if got := img.Bounds().Dy(); got != size {
			t.Errorf("height = %d, want %d", got, size)
		}

		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Errorf("size %d: corner alpha = %d, want 0 (transparent)", size, a)
		}
	}
}

func TestMinSizeHasInk(t *testing.T) {
	// The radius floor exists so an 8px avatar with heavy padding is not blank.
	// Both extremes of the padding range are checked.
	for _, padding := range []float64{0, DefaultPaddingPct, MaxPaddingPct} {
		for _, id := range testIDs[:6] {
			data := mustPNG(t, id, MinSize, padding)

			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			ink := 0
			for y := 0; y < MinSize; y++ {
				for x := 0; x < MinSize; x++ {
					if _, _, _, a := img.At(x, y).RGBA(); a>>8 > 16 {
						ink++
					}
				}
			}
			if ink == 0 {
				t.Errorf("id %q padding %v: %dpx avatar is blank", id, padding, MinSize)
			}
		}
	}
}

func TestPaddingRespected(t *testing.T) {
	for _, padding := range []float64{0, 0.02, 0.05, 0.10, 0.20} {
		for _, size := range []int{64, 128, 256, 512} {
			for _, id := range testIDs[:4] {
				data, err := GeneratePNG(id, size, padding)
				if err != nil {
					t.Fatalf("id %q size %d padding %v: %v", id, size, padding, err)
				}

				minPad, touchesEdge := inkBounds(t, data, size)
				if touchesEdge {
					t.Errorf("id %q size %d padding %v: ink touches the canvas edge", id, size, padding)
				}
				// Padding is a floor: BodyScale adds up to 2.5% extra inset.
				if minPad+1e-9 < padding*100 {
					t.Errorf("id %q size %d padding %v: measured %.2f%%", id, size, padding, minPad)
				}
			}
		}
	}
}

func TestClamping(t *testing.T) {
	// Checked through the clamp helpers so the test does not have to render a
	// 4096px image (about a second and a 64 MiB buffer) to prove the bound.
	if got := clampSize(-100); got != MinSize {
		t.Errorf("clampSize(-100) = %d, want %d", got, MinSize)
	}
	if got := clampSize(MaxSize + 1); got != MaxSize {
		t.Errorf("clampSize(MaxSize+1) = %d, want %d", got, MaxSize)
	}
	if got := clampSize(DefaultSize); got != DefaultSize {
		t.Errorf("clampSize(DefaultSize) = %d, want %d", got, DefaultSize)
	}
	if got := clampPadding(-5); got != 0 {
		t.Errorf("clampPadding(-5) = %v, want 0", got)
	}
	if got := clampPadding(99); got != MaxPaddingPct {
		t.Errorf("clampPadding(99) = %v, want %v", got, MaxPaddingPct)
	}

	// And confirm the clamp actually reaches the renderer.
	small, err := GeneratePNG("clamp", 1, -5)
	if err != nil {
		t.Fatalf("GeneratePNG: %v", err)
	}
	if !bytes.Equal(small, mustPNG(t, "clamp", MinSize, 0)) {
		t.Error("size below MinSize and negative padding were not clamped to the documented values")
	}

	huge, err := GeneratePNG("clamp", 5000, 99)
	if err != nil {
		t.Fatalf("GeneratePNG: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(huge))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if img.Bounds().Dx() != MaxSize || img.Bounds().Dy() != MaxSize {
		t.Errorf("size %d did not clamp to %dx%d", 5000, MaxSize, MaxSize)
	}
}

func TestIDTooLong(t *testing.T) {
	long := make([]byte, MaxIDLength+1)
	for i := range long {
		long[i] = 'a'
	}

	if _, err := GeneratePNG(string(long), 128, DefaultPaddingPct); !errors.Is(err, ErrIDTooLong) {
		t.Errorf("err = %v, want ErrIDTooLong", err)
	}
}

func TestIDAtMaxLength(t *testing.T) {
	ok := make([]byte, MaxIDLength)
	for i := range ok {
		ok[i] = 'a'
	}

	if _, err := GeneratePNG(string(ok), 64, DefaultPaddingPct); err != nil {
		t.Errorf("id at exactly MaxIDLength: %v", err)
	}
}

func TestETag(t *testing.T) {
	a := ETag("abc", 256, 0.05)
	if a != ETag("abc", 256, 0.05) {
		t.Error("ETag is not stable for identical inputs")
	}
	if a == ETag("abc", 128, 0.05) {
		t.Error("ETag does not vary with size")
	}
	if a == ETag("abc", 256, 0.10) {
		t.Error("ETag does not vary with padding")
	}
	if a == ETag("abd", 256, 0.05) {
		t.Error("ETag does not vary with id")
	}
	// quoted, ready for the header
	if len(a) < 3 || a[0] != '"' || a[len(a)-1] != '"' {
		t.Errorf("ETag %q is not correctly quoted", a)
	}
	// clamping must not produce spurious variants
	if ETag("abc", 1, -1) != ETag("abc", MinSize, 0) {
		t.Error("ETag ignores documented clamping")
	}
}

func TestVariantCoverage(t *testing.T) {
	shapes := map[shapeKind]bool{}
	faces := map[faceKind]bool{}
	eyes := map[eyeKind]bool{}

	for i := 0; i < 400; i++ {
		spec := specFromID("variant-" + string(rune('a'+i%26)) + string(rune('a'+i/26)))
		shapes[spec.Kind] = true
		faces[spec.Face] = true
		eyes[spec.Eye] = true
	}

	if len(shapes) != int(shapeKindCount) {
		t.Errorf("only %d/%d shapes reached: %v", len(shapes), shapeKindCount, shapes)
	}
	if len(faces) != int(faceKindCount) {
		t.Errorf("only %d/%d faces reached", len(faces), faceKindCount)
	}
	if len(eyes) != int(eyeKindCount) {
		t.Errorf("only %d/%d eye styles reached", len(eyes), eyeKindCount)
	}
}

func TestSpecInvariants(t *testing.T) {
	for i := 0; i < 200; i++ {
		spec := specFromID("invariant-" + string(rune('a'+i%26)) + string(rune('a'+i/26)))

		if spec.Freq1 != float64(int(spec.Freq1)) || spec.Freq2 != float64(int(spec.Freq2)) {
			t.Fatalf("wobble frequency must be integral to avoid a seam, got %v/%v", spec.Freq1, spec.Freq2)
		}
		if spec.BodyLight <= 0 || spec.BodyLight >= 1 || spec.FaceLight <= 0 || spec.FaceLight >= 1 {
			t.Fatalf("lightness out of range: body %v face %v", spec.BodyLight, spec.FaceLight)
		}
		if spec.FaceLight >= spec.BodyLight {
			t.Fatalf("face must be darker than body: body %v face %v", spec.BodyLight, spec.FaceLight)
		}
		if spec.EyeW <= 0 || spec.EyeH <= 0 || spec.EyeGap <= 0 {
			t.Fatalf("non-positive eye geometry: %v %v %v", spec.EyeW, spec.EyeH, spec.EyeGap)
		}
	}
}

func BenchmarkGeneratePNG256(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id := testIDs[i%len(testIDs)]
		if _, err := GeneratePNG(id, 256, DefaultPaddingPct); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGeneratePNG512(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := GeneratePNG(testIDs[0], 512, DefaultPaddingPct); err != nil {
			b.Fatal(err)
		}
	}
}

func mustPNG(t *testing.T, id string, size int, padding float64) []byte {
	t.Helper()
	data, err := GeneratePNG(id, size, padding)
	if err != nil {
		t.Fatalf("GeneratePNG(%q, %d, %v): %v", id, size, padding, err)
	}
	return data
}

// inkBounds returns the smallest transparent border as a percentage of width,
// ignoring nearly invisible alpha so the antialiasing fringe is not counted.
func inkBounds(t *testing.T, pngBytes []byte, size int) (minPad float64, touchesEdge bool) {
	t.Helper()

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	const inkThreshold = 16 // out of 255
	minX, minY, maxX, maxY := size, size, -1, -1
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a>>8 <= inkThreshold {
				continue
			}
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
	}

	if maxX < 0 {
		return 100, false // empty image
	}

	minPad = float64(min(
		min(minX, minY),
		min(size-1-maxX, size-1-maxY),
	)) / float64(size) * 100

	return minPad, minX == 0 || minY == 0 || maxX == size-1 || maxY == size-1
}

