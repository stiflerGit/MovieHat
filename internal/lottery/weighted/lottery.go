// Package weighted implements the lottery domain service: drawing a winner
// among user IDs and recording the fair-share result of each draw.
package weighted

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"

	"github.com/stiflerGit/moviehat/internal/lottery"
	"github.com/stiflerGit/moviehat/internal/lottery/weighted/scorer"
	"github.com/stiflerGit/moviehat/internal/lottery/weighted/scorer/fair_share/persistence"
	"github.com/stiflerGit/moviehat/pkg/math"
)

// WeightedScorer assigns a relative weight to a Scorer: scorers with higher
// weight contribute more to the draw probabilities.
type WeightedScorer struct {
	// Weight is the scorer's weight relative to the other scorers.
	Weight float64
	// Scorer provides the scores.
	Scorer scorer.Interface
}

// Lottery draws a winner among user IDs and records the fair-share
// accounting of each draw.
type Lottery struct {
	store          persistence.TransactionalStorage
	weightedScorer []WeightedScorer
	logger         *slog.Logger
}

var _ interface {
	lottery.Drawer
	lottery.DrawStorage
} = (*Lottery)(nil)

// New returns a Lottery blending the given scorers. Weights are relative
// and are normalized; an all-zero set of weights is an error.
func New(store persistence.TransactionalStorage, weightedScorers []WeightedScorer, options ...Option) (*Lottery, error) {
	weights := make([]float64, 0, len(weightedScorers))
	for _, wpp := range weightedScorers {
		weights = append(weights, wpp.Weight)
	}

	if math.IsZero(weights) {
		return nil, errors.New("weights are zero")
	}
	math.Normalize(weights)

	weightedScorers = slices.Clone(weightedScorers)

	for i := range weightedScorers {
		weightedScorers[i].Weight = weights[i]
	}

	e := &Lottery{store: store, weightedScorer: weightedScorers, logger: slog.Default()}

	for _, opt := range options {
		opt(e)
	}

	return e, nil
}

// Draw returns the index, in userIDs, of the drawn winner. Draw
// probabilities are those of GetProbabilities.
func (l *Lottery) Draw(ctx context.Context, arg lottery.DrawArg) (lottery.DrawRet, error) {
	if len(arg.UserIDs) == 0 {
		return lottery.DrawRet{}, fmt.Errorf("empty input user ids")
	}

	if len(arg.UserIDs) == 1 {
		return lottery.DrawRet{UserID: arg.UserIDs[0], Index: 0}, nil
	}

	probabilities, err := l.probabilities(ctx, arg.UserIDs)
	if err != nil {
		return lottery.DrawRet{}, fmt.Errorf("e.GetProbabilities: %w", err)
	}

	extraction := rand.Float64()
	cumulativeProbability := float64(0.0)
	for i, probability := range probabilities {
		cumulativeProbability += probability
		if extraction < cumulativeProbability {
			return lottery.DrawRet{UserID: arg.UserIDs[i], Index: i}, nil
		}
	}

	// theoretically we should never reach this point
	// However GetProbabilities returns floats that can sum to 0.999…8 (e.g. 3 equal participants),
	// so a draw in that last sliver falls through: return the last user as fallback
	return lottery.DrawRet{UserID: arg.UserIDs[len(arg.UserIDs)-1], Index: len(arg.UserIDs) - 1}, nil
}

// StoreExtraction records the fair-share accounting of a completed draw:
// each participant gains an equal share of a point, the winner loses one point.
// This is what makes frequent winners less likely to be drawn again.
func (e *Lottery) StoreDrawResult(ctx context.Context, arg lottery.StoreDrawResultArg) (lottery.StoreDrawResultRet, error) {
	if arg.WinnerID == "" {
		return lottery.StoreDrawResultRet{}, errors.New("winner is empty")
	}

	if len(arg.UserIDs) == 0 {
		return lottery.StoreDrawResultRet{}, errors.New("no participants")
	}

	// TODO: This feels a concept of fair-share scorer. Is this concept part of the lottery? or is it part of the fair-share Scorer?
	err := e.store.WithTx(ctx, func(ctx context.Context, storage persistence.Storage) error {
		sessionShare := sessionShare(arg.UserIDs)
		for _, p := range arg.UserIDs {
			_, err := storage.IncreaseUserScore(ctx, persistence.IncreaseUserScoreArg{UserID: p, Inc: sessionShare})
			if err != nil {
				e.logger.InfoContext(ctx, "StoreExtraction r.IncreaseUserScore participant", "error", err)
				return fmt.Errorf("storage.IncreaseUserScore: %w", err)
			}
		}

		_, err := storage.IncreaseUserScore(ctx, persistence.IncreaseUserScoreArg{UserID: arg.WinnerID, Inc: -1.0})
		if err != nil {
			e.logger.InfoContext(ctx, "StoreExtraction r.IncreaseUserScore winner", "error", err)
			return fmt.Errorf("storage.IncreaseUserScore: %w", err)
		}
		return nil
	})
	if err != nil {
		return lottery.StoreDrawResultRet{}, err
	}

	return lottery.StoreDrawResultRet{}, nil
}

// GetProbabilities returns the probability that each user ID is drawn, in
// the same order as userIDs. A single user is drawn with certainty; a scorer
// whose scores are all zero contributes equal probabilities to everyone.
func (l *Lottery) GetProbabilities(ctx context.Context, arg lottery.GetProbabilitiesArg) (lottery.GetProbabilitiesRet, error) {
	if len(arg.UserIDs) == 0 {
		return lottery.GetProbabilitiesRet{}, nil
	}

	if len(arg.UserIDs) == 1 {
		return lottery.GetProbabilitiesRet{Probabilities: []float64{1.0}}, nil
	}

	probabilities, err := l.probabilities(ctx, arg.UserIDs)
	if err != nil {
		return lottery.GetProbabilitiesRet{}, fmt.Errorf("l.probabilities: %w", err)
	}

	return lottery.GetProbabilitiesRet{Probabilities: probabilities}, nil
}

func (l *Lottery) probabilities(ctx context.Context, userIDs []string) ([]float64, error) {
	probabilities := make([]float64, len(userIDs))
	for _, ws := range l.weightedScorer {
		scores, err := ws.Scorer.Scores(ctx, userIDs)
		if err != nil {
			return nil, fmt.Errorf("wpp.ProbabilityProvider.Proabilities: %w", err)
		}

		isZero := math.IsZero(scores)
		zeroValue := float64(float64(1.0) / float64(len(userIDs)))
		if !isZero {
			math.Normalize(scores)
		}

		for i, p := range scores {
			v := zeroValue
			if !isZero {
				v = p
			}
			probabilities[i] += ws.Weight * v
		}
	}

	return probabilities, nil
}

// sessionShare returns the fair-share fraction of a point each participant
// gains per completed draw: with N participants, each gains 1/N.
func sessionShare(userIDs []string) float64 {
	return float64(float64(1) / float64(len(userIDs)))
}
