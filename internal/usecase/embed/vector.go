package embed

import (
	"math"
)

// truncateNormalize slices a vector to dim (when smaller) and L2-renormalizes
// it, returning a fresh slice. dim <= 0 or >= len(v) keeps the full vector.
func truncateNormalize(v []float32, dim int) []float32 {
	n := len(v)
	if dim > 0 && dim < n {
		n = dim
	}
	out := make([]float32, n)
	copy(out, v[:n])

	var sum float64
	for _, x := range out {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return out
	}
	norm := math.Sqrt(sum)
	if math.Abs(norm-1) < 1e-12 {
		return out
	}
	inv := float32(1 / norm)
	for i := range out {
		out[i] *= inv
	}
	return out
}

// l2Norm returns the L2 norm of v.
func l2Norm(v []float32) float64 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	return math.Sqrt(sum)
}

// estimateTextTokens is a pre-engine heuristic for budget enforcement.
func estimateTextTokens(s string) int {
	if s == "" {
		return 0
	}
	t := len(s) / bytesPerTextToken
	if t < 1 {
		t = 1
	}
	return t
}
