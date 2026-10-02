package scorer

import "context"

//go:generate mockgen -package mocks -destination mocks/scorer.go -source=./interface.go

// Interface provides the per-user scores that drive draw probabilities.
type Interface interface {
	// Name returns the scorer's name.
	Name() string
	// Scores returns the score of each user ID, in the same order as userIDs.
	Scores(ctx context.Context, userIDs []string) ([]float64, error)
}
