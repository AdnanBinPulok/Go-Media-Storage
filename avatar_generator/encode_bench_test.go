package avatar_generator

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/klauspost/compress/zlib"
)

// BenchmarkFilterRows1024 measures only the premultiply + Up-filter pass.
func BenchmarkFilterRows1024(b *testing.B) {
	img := renderImage(specFromID(testIDs[0]), 1024, DefaultPaddingPct)
	buf := make([]byte, (1024*4+1)*1024)
	b.ReportAllocs()
	b.SetBytes(int64(len(buf)))

	for i := 0; i < b.N; i++ {
		filterRows(img, buf)
	}
}

// BenchmarkDeflate1024 measures only zlib on already-filtered scanlines, at
// several levels, so the compression tradeoff is explicit.
func BenchmarkDeflate1024(b *testing.B) {
	img := renderImage(specFromID(testIDs[0]), 1024, DefaultPaddingPct)
	raw := make([]byte, (1024*4+1)*1024)
	filterRows(img, raw)

	for _, level := range []int{zlib.HuffmanOnly, zlib.BestSpeed, 2, 4} {
		b.Run(levelName(level), func(b *testing.B) {
			var out bytes.Buffer
			out.Grow(len(raw) / 4)
			zw, err := zlib.NewWriterLevel(&out, level)
			if err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(raw)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				out.Reset()
				zw.Reset(&out)
				zw.Write(raw)
				zw.Close()
			}

			b.StopTimer()
			b.ReportMetric(float64(out.Len()), "bytes")
		})
	}
}

func levelName(level int) string {
	switch level {
	case zlib.HuffmanOnly:
		return "huffman"
	case zlib.BestSpeed:
		return "level1"
	default:
		return "level" + strconv.Itoa(level)
	}
}
