package avatar_generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWritePreviews renders a contact sheet for visual review. It only runs when
// AVATAR_PREVIEW_DIR is set, so normal `go test` stays fast:
//
//	AVATAR_PREVIEW_DIR=./preview go test ./avatar_generator -run TestWritePreviews
func TestWritePreviews(t *testing.T) {
	dir := os.Getenv("AVATAR_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set AVATAR_PREVIEW_DIR to write preview images")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var tiles strings.Builder

	// Sample ids, one per realistic input shape.
	samples := append([]string{}, testIDs...)
	samples = append(samples,
		"papa", "quebec", "romeo", "sierra", "tango",
		"9f1c2e3d-4a5b-6c7d-8e9f-0a1b2c3d4e5f",
	)

	for _, id := range samples {
		png, err := GeneratePNG(id, 256, DefaultPaddingPct)
		if err != nil {
			t.Fatalf("GeneratePNG(%q): %v", id, err)
		}
		name := "id-" + sanitize(id) + ".png"
		if err := os.WriteFile(filepath.Join(dir, name), png, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}

		spec := specFromID(id)
		tiles.WriteString(fmt.Sprintf(
			`<figure><img src="%s"><figcaption>%s<br>%s / %s / %s</figcaption></figure>`,
			name, id, spec.Kind, spec.Face, spec.Eye,
		))
	}

	// Every shape x face x eye combination, so each family is reviewable.
	for _, sh := range []shapeKind{shapeBlob, shapeBox, shapeDrop, shapeLeaf, shapePebble} {
		for _, fc := range []faceKind{faceFull, faceBand, faceOffset, faceHalf} {
			for _, ey := range []eyeKind{eyeBars, eyeSquares, eyeCircles, eyePills, eyeDiamonds, eyeVisor} {
				spec := specFromID(fmt.Sprintf("variant-%d-%d-%d", sh, fc, ey))
				spec.Kind, spec.Face, spec.Eye = sh, fc, ey
				spec.normalizeKinds()
				spec.EyeCol = contrastColor(spec.faceColor())

				png, err := renderPNG(spec, 140, DefaultPaddingPct)
				if err != nil {
					t.Fatalf("renderPNG(%s/%s/%s): %v", sh, fc, ey, err)
				}
				name := fmt.Sprintf("variant-%s-%s-%s.png", sh, fc, ey)
				if err := os.WriteFile(filepath.Join(dir, name), png, 0o644); err != nil {
					t.Fatalf("write: %v", err)
				}
				tiles.WriteString(fmt.Sprintf(
					`<figure><img src="%s"><figcaption>%s / %s / %s</figcaption></figure>`,
					name, sh, fc, ey,
				))
			}
		}
	}

	// Padding variants on one fixed id.
	for _, pct := range []float64{0, 0.02, 0.05, 0.10, 0.20, 0.35} {
		png, err := GeneratePNG("padding-demo", 200, pct)
		if err != nil {
			t.Fatalf("padding %v: %v", pct, err)
		}
		name := fmt.Sprintf("padding-%03.0f.png", pct*100)
		if err := os.WriteFile(filepath.Join(dir, name), png, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		tiles.WriteString(fmt.Sprintf(
			`<figure><img src="%s"><figcaption>padding %.0f%%</figcaption></figure>`,
			name, pct*100,
		))
	}

	html := `<!doctype html><meta charset="utf-8"><title>avatars</title>
<style>
body{background:#20242b;color:#e8e8e8;font:12px system-ui;display:flex;flex-wrap:wrap;gap:14px;padding:24px}
figure{margin:0;text-align:center}
img{width:120px;height:120px;background:#2e333c;border-radius:14px}
figcaption{max-width:120px;overflow-wrap:anywhere;opacity:.62;margin-top:5px}
</style>` + tiles.String()

	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
}

func sanitize(id string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			return r
		default:
			return '_'
		}
	}, id)
}
