package embed

import (
	"math"
	"testing"
)

func TestTruncateNormalize(t *testing.T) {
	v := make([]float32, 768)
	for i := range v {
		v[i] = float32(i%7) + 0.5
	}
	for _, dim := range []int{768, 512, 256, 128} {
		out := truncateNormalize(v, dim)
		if len(out) != dim {
			t.Fatalf("dim %d: got len %d", dim, len(out))
		}
		if n := l2Norm(out); math.Abs(n-1) > 1e-6 {
			t.Fatalf("dim %d: norm %v not within 1e-6 of 1", dim, n)
		}
	}
}

func TestTruncateNormalizeZeroVector(t *testing.T) {
	out := truncateNormalize([]float32{0, 0, 0, 0}, 2)
	if len(out) != 2 {
		t.Fatalf("len = %d", len(out))
	}
	if out[0] != 0 || out[1] != 0 {
		t.Fatalf("zero vector changed: %v", out)
	}
}

func TestEstimateTextTokens(t *testing.T) {
	if got := estimateTextTokens(""); got != 0 {
		t.Fatalf("empty = %d", got)
	}
	if got := estimateTextTokens("a"); got != 1 {
		t.Fatalf("single = %d", got)
	}
	if got := estimateTextTokens("12345678"); got != 2 {
		t.Fatalf("8 bytes = %d, want 2", got)
	}
}

func TestValidDimension(t *testing.T) {
	for _, d := range []int{0, 768, 512, 256, 128} {
		if !validDimension(d) {
			t.Fatalf("%d should be valid", d)
		}
	}
	for _, d := range []int{1, 100, 769, 1024, -1} {
		if validDimension(d) {
			t.Fatalf("%d should be invalid", d)
		}
	}
}
