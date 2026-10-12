package media

import (
	"bytes"
	"strings"
)

// MIME types the service understands.
const (
	MIMEJPEG = "image/jpeg"
	MIMEPNG  = "image/png"
	MIMEWebP = "image/webp"

	MIMEWAV  = "audio/wav"
	MIMEMP3  = "audio/mpeg"
	MIMEOGG  = "audio/ogg"
	MIMEM4A  = "audio/mp4"
	MIMEWebM = "audio/webm"

	MIMEMP4Video  = "video/mp4"
	MIMEWebMVideo = "video/webm"
	MIMEMatroska  = "video/x-matroska"
)

// DetectMime sniffs the media type from magic bytes, returning "" if unknown.
func DetectMime(data []byte) string {
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return MIMEJPEG
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return MIMEPNG
	case len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return MIMEWebP
	case len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WAVE")):
		return MIMEWAV
	case len(data) >= 4 && bytes.Equal(data[0:4], []byte("OggS")):
		return MIMEOGG
	case len(data) >= 3 && bytes.Equal(data[0:3], []byte("ID3")):
		return MIMEMP3
	case len(data) >= 2 && data[0] == 0xFF && (data[1]&0xE0) == 0xE0:
		return MIMEMP3
	case len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")):
		return MIMEM4A
	case len(data) >= 4 && bytes.Equal(data[0:4], []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return MIMEWebM
	default:
		return ""
	}
}

// NormalizeDeclared maps a declared type or audio format shorthand to a MIME.
func NormalizeDeclared(declared string) string {
	d := strings.ToLower(strings.TrimSpace(declared))
	if i := strings.IndexByte(d, ';'); i >= 0 {
		d = strings.TrimSpace(d[:i])
	}
	switch d {
	case "jpeg", "jpg", MIMEJPEG:
		return MIMEJPEG
	case "png", MIMEPNG:
		return MIMEPNG
	case "webp", MIMEWebP:
		return MIMEWebP
	case "wav", "wave", MIMEWAV:
		return MIMEWAV
	case "mp3", MIMEMP3:
		return MIMEMP3
	case "ogg", "opus", "oga", MIMEOGG:
		return MIMEOGG
	case "m4a", "mp4", "aac", MIMEM4A:
		return MIMEM4A
	case "webm", "mkv", "matroska", MIMEWebM:
		return MIMEWebM
	default:
		return ""
	}
}

// IsImage reports whether mime is a supported image type.
func IsImage(mime string) bool {
	switch mime {
	case MIMEJPEG, MIMEPNG, MIMEWebP:
		return true
	}
	return false
}

// IsAudio reports whether mime is a supported audio type.
func IsAudio(mime string) bool {
	switch mime {
	case MIMEWAV, MIMEMP3, MIMEOGG, MIMEM4A, MIMEWebM:
		return true
	}
	return false
}

// DetectVideo sniffs a container type for the video pipeline, returning "" if
// the bytes are not a recognised MP4 or Matroska/WebM container.
func DetectVideo(data []byte) string {
	switch {
	case len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")):
		return MIMEMP4Video
	case len(data) >= 4 && bytes.Equal(data[0:4], []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return MIMEWebMVideo
	default:
		return ""
	}
}

// IsVideo reports whether mime is a supported video container type.
func IsVideo(mime string) bool {
	switch mime {
	case MIMEMP4Video, MIMEWebMVideo, MIMEMatroska:
		return true
	}
	return false
}
