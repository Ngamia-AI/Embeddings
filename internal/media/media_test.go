package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestDetectMime(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0}, MIMEJPEG},
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, MIMEPNG},
		{"wav", []byte("RIFF\x00\x00\x00\x00WAVE"), MIMEWAV},
		{"ogg", []byte("OggS\x00\x00"), MIMEOGG},
		{"mp3", []byte("ID3\x04\x00"), MIMEMP3},
		{"unknown", []byte("hello"), ""},
	}
	for _, c := range cases {
		if got := DetectMime(c.data); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestNormalizeImageDownscalesAndReencodes(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3000, 1500))
	for x := 0; x < 3000; x++ {
		for y := 0; y < 1500; y += 3 {
			src.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}

	n := New(Options{MaxPixels: 40_000_000, MaxSide: 16384, Downscale: 2048, JPEGQuality: 95})
	out, mime, err := n.NormalizeImage(buf.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if mime != MIMEJPEG {
		t.Fatalf("mime = %s", mime)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width > 2048 || cfg.Height > 2048 {
		t.Fatalf("not downscaled: %dx%d", cfg.Width, cfg.Height)
	}
	if cfg.Width != 2048 {
		t.Fatalf("longest side = %d, want 2048", cfg.Width)
	}
	// Must still be a valid JPEG.
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("re-encoded jpeg invalid: %v", err)
	}
}

func TestNormalizeImagePixelLimit(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	n := New(Options{MaxPixels: 1000, MaxSide: 10000, Downscale: 2048, JPEGQuality: 95})
	_, _, err := n.NormalizeImage(buf.Bytes(), "image/png")
	if !PixelError(err) {
		t.Fatalf("err = %v, want pixel error", err)
	}
}

func TestNormalizeImageDeclaredMismatch(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 10, 10))
	var buf bytes.Buffer
	_ = png.Encode(&buf, src)
	n := New(Options{Downscale: 2048})
	_, _, err := n.NormalizeImage(buf.Bytes(), "image/jpeg")
	if err == nil {
		t.Fatal("want mismatch error")
	}
}

func TestWavSeconds(t *testing.T) {
	// 2 seconds of 16 kHz mono 16-bit PCM.
	pcm := 2 * 32000
	data := make([]byte, 44+pcm)
	copy(data, "RIFF")
	copy(data[8:], "WAVE")
	copy(data[12:], "fmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(pcm))
	got := wavSeconds(data)
	if got < 1.99 || got > 2.01 {
		t.Fatalf("wavSeconds = %v, want ~2", got)
	}
	_ = context.Background()
}
