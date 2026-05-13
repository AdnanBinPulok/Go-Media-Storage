package classes

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-media-storage/database_tables"

	"golang.org/x/image/draw"
)

type CompressionRequest struct {
	FileID        string `json:"file_id"`
	Quality       int    `json:"quality"`
	MaxWidth      int    `json:"max_width"`
	Lossless      bool   `json:"lossless"`
	ConvertToWebP bool   `json:"convert_to_webp"`
}

type compressionMeta struct {
	OriginalSize   int64   `json:"original_size"`
	CompressedSize int64   `json:"compressed_size"`
	Ratio          float64 `json:"ratio"`
	Algorithm      string  `json:"algorithm"`
	Quality        int     `json:"quality"`
	MaxWidth       int     `json:"max_width"`
	Lossless       bool    `json:"lossless"`
	CompressedAt   string  `json:"compressed_at"`
}

// RunImageCompression is the entry point. It resolves the file, encodes it,
// writes it atomically, and updates the database record.
func RunImageCompression(mediaFile database_tables.MediaFileModel, req CompressionRequest) {
	if FileManagerInstance == nil {
		log.Printf("[compress] file manager not initialized for %s", mediaFile.StoredName)
		return
	}

	absPath, err := FileManagerInstance.ResolvePath(mediaFile.StoragePath)
	if err != nil {
		log.Printf("[compress] resolve path for %s: %v", mediaFile.StoredName, err)
		return
	}

	f, err := os.Open(absPath)
	if err != nil {
		log.Printf("[compress] open file %s: %v", mediaFile.StoredName, err)
		return
	}

	var (
		compressedBytes []byte
		outExt, outMime = mediaFile.FileExt, mediaFile.MimeType
		algorithm       string
	)

	if strings.ToLower(mediaFile.MimeType) == "image/gif" {
		compressedBytes, algorithm, err = encodeGIF(f, req)
	} else {
		compressedBytes, outExt, outMime, algorithm, err = encodeStillImage(f, mediaFile.MimeType, mediaFile.FileExt, req)
	}
	if err != nil {
		log.Printf("[compress] encode %s: %v", mediaFile.StoredName, err)
		return
	}

	// stored_name never changes to preserve the file URL
	outStoragePath := mediaFile.StoragePath
	outAbsPath := absPath
	if outExt != mediaFile.FileExt {
		outStoragePath = strings.TrimSuffix(mediaFile.StoragePath, mediaFile.FileExt) + outExt
		outAbsPath = strings.TrimSuffix(absPath, mediaFile.FileExt) + outExt
	}

	if err := persistCompression(mediaFile, absPath, outAbsPath, outStoragePath, outExt, outMime, algorithm, compressedBytes, req); err != nil {
		log.Printf("[compress] persist %s: %v", mediaFile.StoredName, err)
	}
}

// encodeGIF decodes every frame of an animated (or static) GIF, scales each
// frame proportionally if max_width is set, and re-encodes to GIF.
// GIF is indexed-colour and inherently lossless; the lossless flag only
// affects the algorithm label recorded in metadata.
func encodeGIF(f *os.File, req CompressionRequest) (data []byte, algorithm string, err error) {
	g, err := gif.DecodeAll(f)
	f.Close()
	if err != nil {
		return nil, "", err
	}

	if req.MaxWidth > 0 && g.Config.Width > req.MaxWidth {
		scaleRatio := float64(req.MaxWidth) / float64(g.Config.Width)
		newCanvasH := max(1, int(float64(g.Config.Height)*scaleRatio))

		for i, frame := range g.Image {
			newW := max(1, int(float64(frame.Bounds().Dx())*scaleRatio))
			newH := max(1, int(float64(frame.Bounds().Dy())*scaleRatio))
			newX := int(float64(frame.Rect.Min.X) * scaleRatio)
			newY := int(float64(frame.Rect.Min.Y) * scaleRatio)

			scaled := scalePalettedToSize(frame, newW, newH)
			scaled.Rect = image.Rect(newX, newY, newX+newW, newY+newH)
			g.Image[i] = scaled
		}

		g.Config.Width = req.MaxWidth
		g.Config.Height = newCanvasH
	}

	algorithm = "gif"
	if req.Lossless {
		algorithm = "gif-lossless"
	}

	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), algorithm, nil
}

// encodeStillImage decodes a single-frame image (JPEG, PNG, or other) and
// re-encodes it. When lossless is true the output is always PNG regardless of
// the source format; otherwise the source format is preserved.
func encodeStillImage(f *os.File, mimeType, originalExt string, req CompressionRequest) (data []byte, outExt, outMime, algorithm string, err error) {
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return nil, "", "", "", err
	}

	if req.MaxWidth > 0 && img.Bounds().Dx() > req.MaxWidth {
		img = scaleImageToWidth(img, req.MaxWidth)
	}

	var buf bytes.Buffer

	if req.ConvertToWebP {
		if webpAvailable {
			outExt = ".webp"
			outMime = "image/webp"
			if req.Lossless {
				algorithm = "webp-lossless"
			} else {
				algorithm = "webp"
			}
			err = encodeWebP(&buf, img, req.Quality, req.Lossless)
		} else {
			// CGo not available — fall back to PNG
			log.Printf("[compress] WebP requires CGo; falling back to PNG")
			outExt = ".png"
			outMime = "image/png"
			algorithm = "png-lossless"
			err = png.Encode(&buf, img)
		}
	} else if req.Lossless {
		outExt = ".png"
		outMime = "image/png"
		algorithm = "png-lossless"
		err = png.Encode(&buf, img)
	} else {
		// Preserve original ext/mime — format is not changing
		outExt = originalExt
		outMime = mimeType
		switch strings.ToLower(mimeType) {
		case "image/jpeg", "image/jpg":
			algorithm = "jpeg"
			err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: req.Quality})
		case "image/png":
			algorithm = "png"
			err = png.Encode(&buf, img)
		default:
			algorithm = "jpeg"
			err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: req.Quality})
		}
	}
	if err != nil {
		return nil, "", "", "", err
	}
	return buf.Bytes(), outExt, outMime, algorithm, nil
}

// persistCompression writes the compressed bytes atomically, removes the old
// file if the format changed, updates the database record, and refreshes vault
// storage statistics.
func persistCompression(
	mediaFile database_tables.MediaFileModel,
	oldAbsPath, newAbsPath, newStoragePath,
	outExt, outMime, algorithm string,
	data []byte,
	req CompressionRequest,
) error {
	if err := atomicWriteFile(oldAbsPath, newAbsPath, data); err != nil {
		return err
	}

	originalSize := mediaFile.FileSizeBytes
	compressedSize := int64(len(data))

	ratio := float64(0)
	if originalSize > 0 {
		ratio = float64(compressedSize) / float64(originalSize) * 100
	}

	meta := compressionMeta{
		OriginalSize:   originalSize,
		CompressedSize: compressedSize,
		Ratio:          ratio,
		Algorithm:      algorithm,
		Quality:        req.Quality,
		MaxWidth:       req.MaxWidth,
		Lossless:       req.Lossless,
		CompressedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	_, err := database_tables.MediaFilesTableInstance.Table.Update(
		map[string]any{
			"compressed":                   true,
			"file_size_bytes":              compressedSize,
			"uncompressed_file_size_bytes": originalSize,
			"compression_metadata":         meta,
			"storage_path":                 newStoragePath,
			"file_ext":                     outExt,
			"mime_type":                    outMime,
		},
		map[string]any{"id": mediaFile.ID},
	)
	if err != nil {
		return err
	}

	log.Printf("[compress] completed %s (%.1f%% of original size)", mediaFile.StoredName, ratio)

	if err := UploadManagerInstance.UpdateVaultMediaStats(mediaFile.VaultID); err != nil {
		log.Printf("[compress] update vault stats for vault %d: %v", mediaFile.VaultID, err)
	}
	return nil
}

func atomicWriteFile(oldAbsPath, newAbsPath string, data []byte) error {
	dirPath := filepath.Dir(oldAbsPath)

	tempFile, err := os.CreateTemp(dirPath, ".tmp-compress-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return err
	}
	if err := tempFile.Chmod(0644); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return err
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		return err
	}

	if err := os.Rename(tempPath, newAbsPath); err != nil {
		os.Remove(tempPath)
		return err
	}

	if newAbsPath != oldAbsPath {
		os.Remove(oldAbsPath)
	}

	return nil
}

func scaleImageToWidth(img image.Image, maxWidth int) image.Image {
	bounds := img.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()
	if origW == 0 {
		return img
	}
	newH := max(1, origH*maxWidth/origW)

	dst := image.NewNRGBA(image.Rect(0, 0, maxWidth, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
	return dst
}

// scalePalettedToSize scales a paletted frame to exactly (newW × newH) pixels.
// Converts to NRGBA for Catmull-Rom quality, then quantises back to the source
// palette using Floyd-Steinberg dithering. The caller is responsible for
// setting the correct Rect (position) on the returned image.
func scalePalettedToSize(src *image.Paletted, newW, newH int) *image.Paletted {
	bounds := src.Bounds()

	nrgba := image.NewNRGBA(bounds)
	draw.Draw(nrgba, bounds, src, bounds.Min, draw.Src)

	scaled := image.NewNRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), nrgba, bounds, draw.Over, nil)

	dst := image.NewPaletted(scaled.Bounds(), src.Palette)
	draw.FloydSteinberg.Draw(dst, dst.Bounds(), scaled, image.Point{})
	return dst
}
