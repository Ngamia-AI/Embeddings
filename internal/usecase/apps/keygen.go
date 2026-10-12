package apps

import (
	"crypto/rand"
	"crypto/sha256"
)

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// newKey returns the plaintext key, its SHA-256 hash and a display prefix.
// Format: emb_<class>_ + 32 random bytes encoded in base62.
func newKey(class string) (plaintext string, hash []byte, prefix string, err error) {
	if class != "bulk" {
		class = "live"
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", nil, "", err
	}
	body := base62Encode(raw)
	plaintext = "emb_" + class + "_" + body
	sum := sha256.Sum256([]byte(plaintext))
	hash = sum[:]
	head := "emb_" + class + "_"
	prefix = plaintext[:len(head)+4]
	return plaintext, hash, prefix, nil
}

// base62Encode encodes bytes as a base62 string.
func base62Encode(src []byte) string {
	if len(src) == 0 {
		return ""
	}
	// Interpret the byte slice as a big-endian integer and divide by 62.
	num := make([]byte, len(src))
	copy(num, src)
	var out []byte
	for {
		rem := 0
		nonzero := false
		for i := range num {
			cur := rem*256 + int(num[i])
			num[i] = byte(cur / 62)
			rem = cur % 62
			if num[i] != 0 {
				nonzero = true
			}
		}
		out = append(out, base62Alphabet[rem])
		if !nonzero {
			break
		}
	}
	// Reverse.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}
