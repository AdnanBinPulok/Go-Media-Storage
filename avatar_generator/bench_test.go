package avatar_generator

import (
	"testing"
)

// The benchmarks cover the sizes that actually reach the route: 256 is the
// default, 1024 is the smallest size that takes the throttled path, and 2048 /
// 4096 are the worst case an embed can request.

func BenchmarkGeneratePNG256(b *testing.B) {
	benchmarkRender(b, testIDs[0], 256)
}

func BenchmarkGeneratePNG512(b *testing.B) {
	benchmarkRender(b, testIDs[0], 512)
}

func BenchmarkGeneratePNG1024(b *testing.B) {
	benchmarkRender(b, testIDs[0], 1024)
}

func BenchmarkGeneratePNG2048(b *testing.B) {
	benchmarkRender(b, testIDs[0], 2048)
}

func benchmarkRender(b *testing.B, id string, size int) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(int64(size * size * 4))

	for i := 0; i < b.N; i++ {
		if _, err := GeneratePNG(id, size, DefaultPaddingPct); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDrawOnly isolates the drawing pass from PNG encoding so a
// regression in either half is visible.
func BenchmarkDrawOnly1024(b *testing.B) {
	spec := specFromID(testIDs[0])
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		img := renderImage(spec, 1024, DefaultPaddingPct)
		if img == nil {
			b.Fatal("nil image")
		}
	}
}

// BenchmarkEncodeOnly1024 measures the PNG write path on a fully drawn image.
func BenchmarkEncodeOnly1024(b *testing.B) {
	img := renderImage(specFromID(testIDs[0]), 1024, DefaultPaddingPct)
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if _, err := encodePNG(img); err != nil {
			b.Fatal(err)
		}
	}
}
