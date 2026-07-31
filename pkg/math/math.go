package math

import (
	"math"
	"slices"
)

// Normalize TODO: document
func Normalize(v []float64) {
	if IsZero(v) {
		return
	}

	minNumber := slices.Min(v)

	sum := float64(0.0)
	for i := range v {
		v[i] += math.Abs(minNumber)
		sum += v[i]
	}

	// sum cannot be zero because IsZero = false
	for i := range v {
		v[i] /= sum
	}
}

func IsZero(v []float64) bool {
	for i := range v {
		if v[i] != 0 {
			return false
		}
	}
	return true
}
