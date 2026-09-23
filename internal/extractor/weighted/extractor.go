package weighted

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/extractor/weighted/scorer/fair_share/persistence"
	"github.com/stiflerGit/moviehat/pkg/math"

	"connectrpc.com/connect"
)

type Scores interface {
	Name() string
	Scores(ctx context.Context, userIDs []string) ([]float64, error)
}

type WeightedScorer struct {
	Weight float64
	Scorer Scores
}

type Extractor struct {
	store          persistence.TransactionalStorage
	weightedScorer []WeightedScorer
	logger         *slog.Logger
}

func New(store persistence.TransactionalStorage, weightedScorers []WeightedScorer, options ...Option) (*Extractor, error) {
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

	e := &Extractor{store: store, weightedScorer: weightedScorers, logger: slog.Default()}

	for _, opt := range options {
		opt(e)
	}

	return e, nil
}

func (e *Extractor) Extract(ctx context.Context, current *pb.Session) (*pb.User, error) {
	probabilities, err := e.GetProbabilities(ctx, current)
	if err != nil {
		return nil, fmt.Errorf("e.GetProbabilities: %w", err)
	}

	extraction := rand.Float64()
	cumulativeProbability := float64(0.0)
	for i, probability := range probabilities {
		cumulativeProbability += probability
		if extraction < cumulativeProbability {
			return current.Participants[i], nil
		}
	}

	// theoretically we should never reach this point
	// However GetProbabilities returns floats that can sum to 0.999…8 (e.g. 3 equal participants),
	// so a rand draw in that last sliver would panic. return current.Participants[len-1] fallback
	return current.Participants[len(current.Participants)-1], nil
}

// StoreExtraction records the score changes for a completed session.
func (e *Extractor) StoreExtraction(ctx context.Context, session *pb.Session) error {
	if session.Winner == nil || session.Winner.Id == "" {
		return errors.New("winner is empty")
	}

	if session.ClosedAt == nil || !session.ClosedAt.IsValid() {
		return errors.New("closedAt is invalid")
	}

	err := e.store.WithTx(ctx, func(ctx context.Context, storage persistence.Storage) error {
		sessionShare := getSessionShare(session)
		for _, p := range session.Participants {
			_, err := storage.IncreaseUserScore(ctx, persistence.IncreaseUserScoreArg{UserID: p.Id, Inc: sessionShare})
			if err != nil {
				e.logger.InfoContext(ctx, "StoreExtraction r.IncreaseUserScore participant", "error", err)
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		_, err := storage.IncreaseUserScore(ctx, persistence.IncreaseUserScoreArg{UserID: session.Winner.Id, Inc: -1.0})
		if err != nil {
			e.logger.InfoContext(ctx, "StoreExtraction r.IncreaseUserScore winner", "error", err)
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (e *Extractor) GetProbabilities(ctx context.Context, current *pb.Session) ([]float64, error) {
	if current == nil || len(current.Participants) == 0 {
		return nil, nil
	}

	if len(current.Participants) == 1 {
		return []float64{1.0}, nil
	}

	userIDs := make([]string, 0, len(current.Participants))
	for _, p := range current.Participants {
		userIDs = append(userIDs, p.Id)
	}

	probabilities := make([]float64, len(current.Participants))
	for _, wpp := range e.weightedScorer {
		scores, err := wpp.Scorer.Scores(ctx, userIDs)
		if err != nil {
			return nil, fmt.Errorf("wpp.ProbabilityProvider.Proabilities: %w", err)
		}

		isZero := math.IsZero(scores)
		zeroValue := float64(float64(1.0) / float64(len(current.Participants)))
		if !isZero {
			math.Normalize(scores)
		}

		for i, p := range scores {
			v := zeroValue
			if !isZero {
				v = p
			}
			probabilities[i] += wpp.Weight * v
		}
	}

	return probabilities, nil
}

func getSessionShare(s *pb.Session) float64 {
	return float64(float64(1) / float64(len(s.Participants)))
}
