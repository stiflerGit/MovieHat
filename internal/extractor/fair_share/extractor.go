package extractor

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/extractor/fair_share/persistence"

	"connectrpc.com/connect"
)

const (
	defaultEqualChanceRate = 0.3
)

// Extractor selects winners using fair-share scoring.
type Extractor struct {
	store           persistence.Store
	equalChanceRate float64
}

// New creates a fair-share extractor.
func New(store persistence.Store, options ...Option) *Extractor {
	e := &Extractor{
		store:           store,
		equalChanceRate: defaultEqualChanceRate,
	}

	for _, opt := range options {
		opt(e)
	}

	return e
}

// Extract selects a winner for the current session.
func (e *Extractor) Extract(ctx context.Context, current *pb.Session) (*pb.User, error) {
	if current == nil || len(current.Participants) == 0 {
		return nil, nil
	}

	userIDToScore, err := e.fetchUserScoresFromRepository(ctx, current)
	if err != nil {
		return nil, fmt.Errorf("e.fetchUserScoresFromRepository: %w", err)
	}

	winnerIndex := e.extract(userIDToScore, current.Participants)

	return current.Participants[winnerIndex], nil
}

// StoreExtraction records the score changes for a completed session.
func (e *Extractor) StoreExtraction(ctx context.Context, session *pb.Session) error {
	if session.Winner == nil || session.Winner.Id == "" {
		return errors.New("winner is empty")
	}

	if session.ClosedAt == nil || !session.ClosedAt.IsValid() {
		return errors.New("closedAt is invalid")
	}

	err := e.store.WithTx(ctx, func(ctx context.Context, r persistence.Repository) error {
		sessionShare := getSessionShare(session)
		for _, p := range session.Participants {
			_, err := r.IncreaseUserScore(ctx, persistence.IncreaseUserScoreArg{UserID: p.Id, Inc: sessionShare})
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		_, err := r.IncreaseUserScore(ctx, persistence.IncreaseUserScoreArg{UserID: session.Winner.Id, Inc: -1.0})
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (e *Extractor) fetchUserScoresFromRepository(ctx context.Context, current *pb.Session) (userIDToScore map[string]float64, err error) {
	userIDs := make([]string, 0, len(current.Participants))
	for _, p := range current.Participants {
		userIDs = append(userIDs, p.Id)
	}

	listUsersScoresRet, err := e.store.ListUsersScores(ctx, persistence.ListUsersScoresArg{UserIDs: userIDs})
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

// ExtractWithHistory selects a winner using the provided session history.
func (e *Extractor) ExtractWithHistory(ctx context.Context, history []*pb.Session, current *pb.Session) (*pb.User, error) {
	if current == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("current session is nil"))
	}

	if len(current.Participants) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no participants in the current session"))
	}

	if len(history) == 0 {
		return current.Participants[e.uniformProbability(len(current.Participants))], nil
	}

	userIDToScore := computeUserScoresFromHistory(history)
	winnerIndex := e.extract(userIDToScore, current.Participants)

	return current.Participants[winnerIndex], nil
}

func (e Extractor) extract(userIDToScore map[string]float64, participants []*pb.User) int {
	if len(participants) == 0 {
		return -1
	}

	if len(participants) == 1 {
		return 0
	}

	maxScore := 0.0
	scores := make([]float64, 0, len(participants))
	for _, p := range participants {
		score := max(0.0, userIDToScore[p.Id])
		maxScore = max(maxScore, score)
		scores = append(scores, score)
	}

	if maxScore == 0 {
		return e.uniformProbability(len(participants))
	}

	normalize(scores)

	// after normalization we can talk about chances
	chances := scores
	if e.equalChanceRate > 0 {
		equalChanceScore := float64(float64(1.0) / float64(len(participants)))
		for i := range chances {
			chances[i] = e.equalChanceRate*equalChanceScore + (1-e.equalChanceRate)*chances[i]
		}
	}

	extraction := rand.Float64()
	cumulativeChance := float64(0.0)
	for i, chance := range chances {
		cumulativeChance += chance
		if extraction < cumulativeChance {
			return i
		}
	}

	panic("should never reach this point")
}

func (e *Extractor) uniformProbability(count int) int {
	return rand.IntN(count)
}

func computeUserScoresFromHistory(history []*pb.Session) map[string]float64 {
	userIDToScore := map[string]float64{}

	for _, s := range history {
		sessionShare := getSessionShare(s)
		for _, u := range s.Participants {
			userIDToScore[u.Id] += sessionShare
		}
		userIDToScore[s.Winner.Id] -= 1.0
	}

	return userIDToScore
}

func normalize(v []float64) {
	sum := float64(0.0)
	for i := range v {
		sum += v[i]
	}

	if sum == 0 {
		return
	}

	for i := range v {
		v[i] /= sum
	}
}

func getSessionShare(s *pb.Session) float64 {
	return float64(float64(1) / float64(len(s.Participants)))
}
