package avatar_generator

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"sync"

	"github.com/klauspost/compress/zlib"
)

// A PNG writer tuned for generated avatars.
//
// image/png tries all five PNG filters on every row, then deflates the winner.
// At 1024px that costs more than drawing the avatar. Avatars are flat, smooth
// shapes on a transparent background, so a fixed Up filter compresses well and
// needs no per-row trial.
//
// The decoded pixels are identical to the stdlib encoder: the premultiplied to
// straight-alpha conversion below copies image/png's own shortcut exactly
// (including its rounding), so switching writers cannot change an image.
const (
	pngFilterUp = 2

	// pngChunkMax keeps each IDAT comfortably sized for conservative decoders.
	pngChunkMax = 32 << 10

	// maxPooledFiltered stops a 4096px render (67MB of scanlines) from being
	// pinned in the pool forever. 24MB covers 2048px, the largest size that is
	// commonly embedded.
	maxPooledFiltered = 24 << 20
)

var (
	pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	pngCRCTable  = crc32.MakeTable(crc32.IEEE)
)

// ErrEmptyImage is returned when the encoder is given a zero-sized image.
var ErrEmptyImage = errors.New("avatar_generator: empty image")

// fastPNGWriter is pooled encoder state. zlib.Writer keeps a large window and
// hash tables, so rebuilding it per avatar would dominate the encode.
type fastPNGWriter struct {
	comp bytes.Buffer
	out  bytes.Buffer
	zw   *zlib.Writer
}

var fastPNGPool = sync.Pool{New: func() any { return new(fastPNGWriter) }}

// filteredPool keeps a few large scanline buffers alive across renders.
//
// sync.Pool is emptied by every GC cycle, which means a 4MB (or 16MB) buffer had
// to be reallocated and zeroed on a regular basis. This small bounded stack is
// not GC-managed, so the steady state is allocation-free while memory stays
// capped at a handful of buffers.
var filteredPool struct {
	mu   sync.Mutex
	free [][]byte
}

const maxFilteredBuffers = 2

func getFiltered(n int) []byte {
	filteredPool.mu.Lock()
	defer filteredPool.mu.Unlock()

	for i := len(filteredPool.free) - 1; i >= 0; i-- {
		buf := filteredPool.free[i]
		if cap(buf) >= n {
			filteredPool.free = append(filteredPool.free[:i], filteredPool.free[i+1:]...)
			return buf[:n]
		}
	}
	return make([]byte, n)
}

func putFiltered(buf []byte) {
	if cap(buf) == 0 || cap(buf) > maxPooledFiltered {
		return
	}

	filteredPool.mu.Lock()
	defer filteredPool.mu.Unlock()

	if len(filteredPool.free) < maxFilteredBuffers {
		filteredPool.free = append(filteredPool.free, buf[:0])
	}
}

// encodePNGFast writes img as an 8-bit RGBA PNG.
func encodePNGFast(img *image.RGBA) ([]byte, error) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if w <= 0 || h <= 0 {
		return nil, ErrEmptyImage
	}

	filtered := getFiltered((w*4 + 1) * h)
	defer putFiltered(filtered)

	filterRows(img, filtered)

	fw := fastPNGPool.Get().(*fastPNGWriter)
	defer fastPNGPool.Put(fw)

	fw.comp.Reset()
	fw.out.Reset()

	if err := fw.compress(filtered); err != nil {
		return nil, err
	}

	fw.out.Write(pngSignature)
	fw.writeIHDR(w, h)
	fw.writeIDAT(fw.comp.Bytes())
	fw.writeChunk("IEND", nil)

	// Copy out: the pool may hand the same buffers to another request.
	out := make([]byte, fw.out.Len())
	copy(out, fw.out.Bytes())
	return out, nil
}

// compress deflates the filtered scanlines into fw.comp.
func (fw *fastPNGWriter) compress(filtered []byte) error {
	if fw.zw == nil {
		zw, err := zlib.NewWriterLevel(&fw.comp, zlib.BestSpeed)
		if err != nil {
			return err
		}
		fw.zw = zw
	} else {
		fw.zw.Reset(&fw.comp)
	}

	if _, err := fw.zw.Write(filtered); err != nil {
		return err
	}
	return fw.zw.Close()
}

func (fw *fastPNGWriter) writeIHDR(w, h int) {
	var data [13]byte
	binary.BigEndian.PutUint32(data[0:4], uint32(w))
	binary.BigEndian.PutUint32(data[4:8], uint32(h))
	data[8] = 8 // bit depth
	data[9] = 6 // color type: truecolor with alpha
	data[10] = 0
	data[11] = 0
	data[12] = 0
	fw.writeChunk("IHDR", data[:])
}

func (fw *fastPNGWriter) writeIDAT(comp []byte) {
	for len(comp) > 0 {
		n := min(len(comp), pngChunkMax)
		fw.writeChunk("IDAT", comp[:n])
		comp = comp[n:]
	}
}

func (fw *fastPNGWriter) writeChunk(typ string, data []byte) {
	var buf [4]byte

	binary.BigEndian.PutUint32(buf[:], uint32(len(data)))
	fw.out.Write(buf[:])

	start := fw.out.Len()
	fw.out.WriteString(typ)
	fw.out.Write(data)

	crc := crc32.Checksum(fw.out.Bytes()[start:], pngCRCTable)
	binary.BigEndian.PutUint32(buf[:], crc)
	fw.out.Write(buf[:])
}

// filterRows converts premultiplied RGBA into Up-filtered PNG scanlines.
// Bands are independent because each starts from the raw previous row.
func filterRows(img *image.RGBA, dst []byte) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	rowLen := w * 4
	stride := img.Stride

	parallelBands(0, h, func(lo, hi int) {
		prev := make([]byte, rowLen)
		if lo > 0 {
			start := (lo - 1) * stride
			straightRow(img.Pix[start:start+rowLen], prev)
		}

		for y := lo; y < hi; y++ {
			src := img.Pix[y*stride : y*stride+rowLen]
			row := dst[y*(rowLen+1) : (y+1)*(rowLen+1)]
			row[0] = pngFilterUp
			straightFiltered(src, row[1:], prev)
		}
	})
}

// straightRow converts one premultiplied row into straight-alpha RGBA.
func straightRow(src, dst []byte) {
	for i := 0; i+3 < len(src); i += 4 {
		straightPixel(src[i:i+4], dst[i:i+4])
	}
}

// straightFiltered converts a row and applies the Up filter in one pass, then
// leaves the converted row in prev for the next call.
//
// The conversion is image/png's own premultiplied -> NRGBA shortcut, inlined:
// the generic version spends a measurable amount of the encode in slice setup.
func straightFiltered(src, body, prev []byte) {
	// m combines 0x101 (8-bit to 16-bit) with 0xffff (the division that undoes
	// premultiplication).
	const m = 0x101 * 0xffff

	for i := 0; i+3 < len(src); i += 4 {
		a := src[i+3]

		var r, g, b uint8
		switch a {
		case 0x00:
			// fully transparent: straight color is zero, r/g/b stay zero
		case 0xff:
			r, g, b = src[i], src[i+1], src[i+2]
		default:
			a16 := uint32(a) * 0x101
			r = uint8(uint32(src[i]) * m / a16 >> 8)
			g = uint8(uint32(src[i+1]) * m / a16 >> 8)
			b = uint8(uint32(src[i+2]) * m / a16 >> 8)
		}

		body[i+0] = r - prev[i+0]
		body[i+1] = g - prev[i+1]
		body[i+2] = b - prev[i+2]
		body[i+3] = a - prev[i+3]

		prev[i+0], prev[i+1], prev[i+2], prev[i+3] = r, g, b, a
	}
}

// straightPixel is image/png's own premultiplied -> NRGBA shortcut, verbatim.
func straightPixel(src, dst []byte) {
	switch a := src[3]; a {
	case 0x00:
		dst[0], dst[1], dst[2], dst[3] = 0, 0, 0, 0
	case 0xff:
		dst[0], dst[1], dst[2], dst[3] = src[0], src[1], src[2], 0xff
	default:
		// m combines 0x101 (8-bit to 16-bit) with 0xffff (the division that
		// undoes premultiplication).
		const m = 0x101 * 0xffff
		a16 := uint32(a) * 0x101
		dst[0] = uint8(uint32(src[0]) * m / a16 >> 8)
		dst[1] = uint8(uint32(src[1]) * m / a16 >> 8)
		dst[2] = uint8(uint32(src[2]) * m / a16 >> 8)
		dst[3] = a
	}
}
