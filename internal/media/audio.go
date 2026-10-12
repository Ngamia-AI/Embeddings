package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os/exec"
)

var errAudioTooLong = errors.New("audio too long")

// AudioTooLong reports whether err is an over-length violation.
func AudioTooLong(err error) bool { return errors.Is(err, errAudioTooLong) }

// NormalizeAudio transcodes the input to mono 16 kHz WAV with ffmpeg, using
// only the pipe protocol, and returns the normalized bytes and duration.
func (n *Normalizer) NormalizeAudio(ctx context.Context, data []byte, format string) ([]byte, float64, error) {
	detected := DetectMime(data)
	if !IsAudio(detected) {
		return nil, 0, errors.New("not a supported audio type")
	}
	declared := NormalizeDeclared(format)
	if declared != "" && declared != detected {
		return nil, 0, fmt.Errorf("declared %s but content is %s", declared, detected)
	}

	cctx, cancel := context.WithTimeout(ctx, n.ffmpegTimeout)
	defer cancel()

	// -t 121 caps the output so a long upload cannot exhaust memory; anything
	// decoded past the 120 s limit is rejected below.
	args := []string{
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-protocol_whitelist", "pipe",
		"-i", "pipe:0",
		"-t", "121",
		"-ac", "1", "-ar", "16000",
		"-f", "wav", "pipe:1",
	}
	cmd := exec.CommandContext(cctx, n.ffmpegPath, args...)
	cmd.Stdin = bytes.NewReader(data)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		if cctx.Err() != nil {
			return nil, 0, fmt.Errorf("ffmpeg timeout: %w", cctx.Err())
		}
		return nil, 0, fmt.Errorf("ffmpeg: %v: %s", err, truncate(errBuf.String(), 200))
	}

	seconds := wavSeconds(out.Bytes())
	if seconds > n.maxAudioSeconds {
		return nil, 0, errAudioTooLong
	}
	return out.Bytes(), seconds, nil
}

// wavSeconds computes duration for mono 16 kHz 16-bit PCM WAV output.
func wavSeconds(b []byte) float64 {
	const bytesPerSecond = 16000 * 1 * 2 // 16 kHz, mono, 16-bit
	if len(b) <= 44 {
		return 0
	}
	// Walk the chunk list to find the data chunk.
	if len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WAVE" {
		i := 12
		for i+8 <= len(b) {
			id := string(b[i : i+4])
			size := int(binary.LittleEndian.Uint32(b[i+4 : i+8]))
			if id == "data" {
				if size < 0 || i+8+size > len(b) {
					size = len(b) - (i + 8)
				}
				return float64(size) / bytesPerSecond
			}
			i += 8 + size + (size & 1)
		}
	}
	return float64(len(b)-44) / bytesPerSecond
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
