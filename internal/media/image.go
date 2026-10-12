package media

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"

	// Register decoders.
	_ "image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

var errImageTooManyPixels = errors.New("image exceeds pixel or side limit")

// PixelError reports whether err is an image size violation.
func PixelError(err error) bool { return errors.Is(err, errImageTooManyPixels) }

// NormalizeImage decodes, bounds-checks, downscales and re-encodes an image as
// JPEG quality 95. The declared MIME must match the magic bytes.
func (n *Normalizer) NormalizeImage(data []byte, declaredMime string) ([]byte, string, error) {
	detected := DetectMime(data)
	if !IsImage(detected) {
		return nil, "", errors.New("not a supported image")
	}
	declared := NormalizeDeclared(declaredMime)
	if declared != "" && declared != detected {
		return nil, "", fmt.Errorf("declared %s but content is %s", declared, detected)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode config: %w", err)
	}
	if n.maxPixels > 0 && cfg.Width*cfg.Height > n.maxPixels {
		return nil, "", errImageTooManyPixels
	}
	if n.maxSide > 0 && (cfg.Width > n.maxSide || cfg.Height > n.maxSide) {
		return nil, "", errImageTooManyPixels
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode: %w", err)
	}

	dst := downscale(src, n.downscale)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: n.jpegQuality}); err != nil {
		return nil, "", fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), MIMEJPEG, nil
}

// downscale scales src so its longest side is at most max, preserving aspect.
func downscale(src image.Image, max int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if max <= 0 || (w <= max && h <= max) {
		return src
	}
	if w >= h {
		h = h * max / w
		w = max
	} else {
		w = w * max / h
		h = max
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}
