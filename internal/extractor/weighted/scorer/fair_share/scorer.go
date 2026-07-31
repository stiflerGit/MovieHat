package extractor

import (
	"context"
	"errors"
	"fmt"
	"maps"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/extractor/weighted/scorer/fair_share/persistence"
)

// Scorer provides probabilities based on fair-share scoring.
type Scorer struct {
	store persistence.TransactionalStorage
}

// New creates a fair-share score provider.
func New(store persistence.TransactionalStorage) *Scorer {
	return &Scorer{store: store}
}

func (p *Scorer) Name() string {
	return "fair_probability"
}

// Extract selects a winner for the current session.
func (p *Scorer) Scores(ctx context.Context, userIDs []string) ([]float64, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	userIDToScore, err := p.fetchUserScoresFromRepository(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("e.fetchUserScoresFromRepository: %w", err)
	}

	scores := make([]float64, 0, len(userIDs))
	for _, id := range userIDs {
		score := max(0.0, userIDToScore[id])
		scores = append(scores, score)
	}

	return scores, nil
}

// StoreExtraction records the score changes for a completed session.
func (p *Scorer) StoreExtraction(ctx context.Context, session *pb.Session) error {
	if session.Winner == nil || session.Winner.Id == "" {
		return errors.New("winner is empty")
	}

	if session.ClosedAt == nil || !session.ClosedAt.IsValid() {
		return errors.New("closedAt is invalid")
	}

	err := p.store.WithTx(ctx, func(ctx context.Context, r persistence.Storage) error {
		sessionShare := getSessionShare(session)
		for _, p := range session.Participants {
			_, err := r.IncreaseUserScore(ctx, persistence.IncreaseUserScoreArg{UserID: p.Id, Inc: sessionShare})
			if err != nil {
				return fmt.Errorf("r.IncreaseUserScore: %w", err)
			}
		}

		_, err := r.IncreaseUserScore(ctx, persistence.IncreaseUserScoreArg{UserID: session.Winner.Id, Inc: -1.0})
		if err != nil {
			return fmt.Errorf("r.IncreaseUserScore winner: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *Scorer) fetchUserScoresFromRepository(ctx context.Context, userIDs []string) (userIDToScore map[string]float64, err error) {
	listUsersScoresRet, err := p.store.ListUsersScores(ctx, persistence.ListUsersScoresArg{UserIDs: userIDs})
	if err != nil {
		return nil, fmt.Errorf("e.store.ListUsersScores: %w", err)
	}

	userIDToScore = maps.Collect(func(yield func(string, float64) bool) {
		for _, uc := range listUsersScoresRet.UserScores {
			if !yield(uc.UserID, uc.Score) {
				return
			}
		}
	})

	return userIDToScore, nil
}

func getSessionShare(s *pb.Session) float64 {
	return float64(float64(1) / float64(len(s.Participants)))
}
