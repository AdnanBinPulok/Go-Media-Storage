package avatar_generator

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// goldenPath stores a digest of the decoded pixels for a fixed id/size matrix.
//
// Digests are taken after PNG decoding, in non-premultiplied 8-bit RGBA, so an
// encoder change (different filters, different deflate) does not invalidate
// them. Every rendering optimization must keep these values unchanged.
const goldenPath = "testdata/pixels.golden"

var goldenIDs = []string{
	"550e8400-e29b-41d4-a716-446655440000",
	"user_42",
	"9f1c2e3d-4a5b-6c7d-8e9f-0a1b2c3d4e5f",
	"team-alpha",
}

var goldenSizes = []int{8, 16, 32, 64, 128, 256, 512, 1024}

// TestPixelGolden locks the rendered pixels. Set UPDATE_GOLDEN=1 to rewrite the
// file (only acceptable when the change is intentional and reviewed).
func TestPixelGolden(t *testing.T) {
	update := os.Getenv("UPDATE_GOLDEN") != ""

	want, err := readGolden()
	if err != nil && !update {
		t.Fatalf("read %s: %v (run with UPDATE_GOLDEN=1 to create it)", goldenPath, err)
	}

	var out bytes.Buffer
	for _, id := range goldenIDs {
		for _, size := range goldenSizes {
			data, err := GeneratePNG(id, size, DefaultPaddingPct)
			if err != nil {
				t.Fatalf("GeneratePNG(%q, %d): %v", id, size, err)
			}

			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("decode %q %d: %v", id, size, err)
			}

			got := pixelDigest(img)
			fmt.Fprintf(&out, "%s\t%d\t%s\n", id, size, got)

			if update {
				continue
			}
			if key := goldenKey(id, size); want[key] != got {
				t.Errorf("%s: pixels changed", key)
			}
		}
	}

	if update {
		writeGolden(t, out.Bytes())
	}
}

func goldenKey(id string, size int) string {
	return id + "\t" + strconv.Itoa(size)
}

func readGolden() (map[string]string, error) {
	f, err := os.Open(goldenPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	lines := make(map[string]string)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Split(sc.Text(), "\t")
		if len(parts) != 3 {
			continue
		}
		lines[parts[0]+"\t"+parts[1]] = parts[2]
	}
	return lines, sc.Err()
}

func writeGolden(t *testing.T, data []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
		t.Fatalf("write golden: %v", err)
	}
	t.Logf("wrote %s", goldenPath)
}

// pixelDigest hashes the image as straight-alpha 8-bit RGBA, row by row.
func pixelDigest(img image.Image) string {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	hsh := sha256.New()

	// The PNG encoder writes color type 6, so decoding normally hands back
	// NRGBA and the raw bytes are already the digest input.
	if m, ok := img.(*image.NRGBA); ok {
		row := make([]byte, 0, w*4)
		for y := 0; y < h; y++ {
			i := m.PixOffset(b.Min.X, b.Min.Y+y)
			row = append(row[:0], m.Pix[i:i+w*4]...)
			hsh.Write(row)
		}
		return hex.EncodeToString(hsh.Sum(nil))
	}

	row := make([]byte, 0, w*4)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row = row[:0]
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			row = append(row, c.R, c.G, c.B, c.A)
		}
		hsh.Write(row)
	}
	return hex.EncodeToString(hsh.Sum(nil))
}
