package equal

import (
	"context"
	"slices"
)

type Scorer struct{}

func (s Scorer) Name() string {
	return "equal_probability"
}

func (s Scorer) Scores(ctx context.Context, userIDs []string) ([]float64, error) {
	probability := float64(float64(1.0) / float64(len(userIDs)))
	return slices.Repeat([]float64{probability}, len(userIDs)), nil
}
