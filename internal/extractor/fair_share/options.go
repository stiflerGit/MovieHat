package extractor

// Option configures a fair-share extractor.
type Option func(*Extractor)

// WithEqualChanceRate sets the equal-chance component of winner selection.
func WithEqualChanceRate(v float64) Option {
	return func(e *Extractor) {
		e.equalChanceRate = v
	}
}
