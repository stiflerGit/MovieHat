package extractor

// Option configures a fair-share extractor.
type Option func(*Extractor)

// WithEqualProbabilityRate sets the equal-probability component of winner selection.
func WithEqualProbabilityRate(v float64) Option {
	return func(e *Extractor) {
		e.equalProbabilityRate = v
	}
}
