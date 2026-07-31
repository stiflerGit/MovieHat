package weighted

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	scoresmock "github.com/stiflerGit/moviehat/internal/extractor/weighted/mocks"
	"github.com/stiflerGit/moviehat/internal/extractor/weighted/scorer/fair_share/persistence"
	storemock "github.com/stiflerGit/moviehat/internal/extractor/weighted/scorer/fair_share/persistence/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func sessionWith(userIDs ...string) *pb.Session {
	s := &pb.Session{}
	for _, id := range userIDs {
		s.Participants = append(s.Participants, &pb.User{Id: id})
	}
	return s
}

// newScorer builds a WeightedScorer whose Scores() returns the given values.
func newScorer(t *testing.T, weight float64, scores []float64, err error) WeightedScorer {
	ctrl := gomock.NewController(t)
	m := scoresmock.NewMockScores(ctrl)
	m.EXPECT().Scores(gomock.Any(), gomock.Any()).Return(scores, err).AnyTimes()
	return WeightedScorer{Weight: weight, Scorer: m}
}

func TestNew(t *testing.T) {
	t.Run("zero weights returns error", func(t *testing.T) {
		_, err := New(nil, []WeightedScorer{{Weight: 0, Scorer: nil}})
		require.Error(t, err)
	})

	t.Run("normalizes weights", func(t *testing.T) {
		// math.Normalize shifts by |min| before dividing: [3,1] -> [4,2]/6.
		e, err := New(nil, []WeightedScorer{{Weight: 3}, {Weight: 1}})
		require.NoError(t, err)
		require.InDelta(t, 4.0/6.0, e.weightedScorer[0].Weight, 1e-9)
		require.InDelta(t, 2.0/6.0, e.weightedScorer[1].Weight, 1e-9)
		require.InDelta(t, 1.0, e.weightedScorer[0].Weight+e.weightedScorer[1].Weight, 1e-9)
	})
}

func TestExtractor_GetProbabilities(t *testing.T) {
	testCases := []struct {
		name    string
		session *pb.Session
		scorer  func(t *testing.T) WeightedScorer
		want    []float64
		wantErr bool
	}{
		{
			name:    "nil session",
			session: nil,
			scorer:  func(t *testing.T) WeightedScorer { return newScorer(t, 1, nil, nil) },
			want:    nil,
		},
		{
			name:    "no participants",
			session: sessionWith(),
			scorer:  func(t *testing.T) WeightedScorer { return newScorer(t, 1, nil, nil) },
			want:    nil,
		},
		{
			name:    "single participant is certain",
			session: sessionWith("u1"),
			scorer:  func(t *testing.T) WeightedScorer { return newScorer(t, 1, nil, nil) },
			want:    []float64{1.0},
		},
		{
			name:    "scorer error propagates",
			session: sessionWith("u1", "u2"),
			scorer:  func(t *testing.T) WeightedScorer { return newScorer(t, 1, nil, errors.New("boom")) },
			wantErr: true,
		},
		{
			name:    "zero scores fall back to uniform",
			session: sessionWith("u1", "u2"),
			scorer:  func(t *testing.T) WeightedScorer { return newScorer(t, 1, []float64{0, 0}, nil) },
			want:    []float64{0.5, 0.5},
		},
		{
			name:    "non-zero scores are normalized",
			session: sessionWith("u1", "u2", "u3"),
			scorer:  func(t *testing.T) WeightedScorer { return newScorer(t, 1, []float64{2, 0, 2}, nil) },
			// Normalize adds |min| (0) then divides by sum(4): 0.5, 0, 0.5
			want: []float64{0.5, 0, 0.5},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e, err := New(nil, []WeightedScorer{tc.scorer(t)})
			require.NoError(t, err)

			got, err := e.GetProbabilities(t.Context(), tc.session)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.InDeltaSlice(t, tc.want, got, 1e-9)
		})
	}
}

func TestExtractor_GetProbabilities_CombinesWeightedScorers(t *testing.T) {
	// weights [3,1] normalize (shift by |min|) to [4/6, 2/6].
	// scorer A: all mass on u1 -> [1, 0]
	// scorer B: uniform fallback -> [0.5, 0.5]
	// expected: [4/6*1 + 2/6*0.5, 4/6*0 + 2/6*0.5] = [5/6, 1/6]
	e, err := New(nil, []WeightedScorer{
		newScorer(t, 3, []float64{1, 0}, nil),
		newScorer(t, 1, []float64{0, 0}, nil),
	})
	require.NoError(t, err)

	got, err := e.GetProbabilities(t.Context(), sessionWith("u1", "u2"))
	require.NoError(t, err)
	require.InDeltaSlice(t, []float64{5.0 / 6.0, 1.0 / 6.0}, got, 1e-9)
}

func TestExtractor_Extract(t *testing.T) {
	t.Run("single participant wins", func(t *testing.T) {
		e, err := New(nil, []WeightedScorer{newScorer(t, 1, nil, nil)})
		require.NoError(t, err)

		winner, err := e.Extract(t.Context(), sessionWith("only"))
		require.NoError(t, err)
		require.Equal(t, "only", winner.Id)
	})

	t.Run("never picks a zero-probability participant and never panics", func(t *testing.T) {
		// u2 has probability 0; running many draws must never return u2 nor panic.
		e, err := New(nil, []WeightedScorer{newScorer(t, 1, []float64{1, 0}, nil)})
		require.NoError(t, err)

		for i := 0; i < 1000; i++ {
			winner, err := e.Extract(t.Context(), sessionWith("u1", "u2"))
			require.NoError(t, err)
			require.Equal(t, "u1", winner.Id)
		}
	})

	t.Run("returns a real participant across many draws", func(t *testing.T) {
		e, err := New(nil, []WeightedScorer{newScorer(t, 1, []float64{1, 1, 1}, nil)})
		require.NoError(t, err)

		for i := 0; i < 1000; i++ {
			winner, err := e.Extract(t.Context(), sessionWith("u1", "u2", "u3"))
			require.NoError(t, err)
			require.Contains(t, []string{"u1", "u2", "u3"}, winner.Id)
		}
	})
}

func TestExtractor_StoreExtraction(t *testing.T) {
	closedSession := func() *pb.Session {
		return &pb.Session{
			Participants: []*pb.User{{Id: "u1"}, {Id: "u2"}},
			Winner:       &pb.User{Id: "u2"},
			ClosedAt:     timestamppb.New(time.Now()),
		}
	}

	t.Run("empty winner returns error", func(t *testing.T) {
		e, err := New(nil, []WeightedScorer{newScorer(t, 1, nil, nil)})
		require.NoError(t, err)

		s := closedSession()
		s.Winner = nil
		require.Error(t, e.StoreExtraction(t.Context(), s))
	})

	t.Run("invalid closedAt returns error", func(t *testing.T) {
		e, err := New(nil, []WeightedScorer{newScorer(t, 1, nil, nil)})
		require.NoError(t, err)

		s := closedSession()
		s.ClosedAt = nil
		require.Error(t, e.StoreExtraction(t.Context(), s))
	})

	t.Run("increments participants by share and winner by -1", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storemock.NewMockTransactionalStorage(ctrl)

		store.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context, persistence.Storage) error) error {
				return fn(ctx, store)
			},
		)

		got := map[string]float64{}
		store.EXPECT().IncreaseUserScore(gomock.Any(), gomock.Any()).Times(3).DoAndReturn(
			func(_ context.Context, arg persistence.IncreaseUserScoreArg) (persistence.IncreaseUserScoreRet, error) {
				got[arg.UserID] += arg.Inc
				return persistence.IncreaseUserScoreRet{}, nil
			},
		)

		e, err := New(store, []WeightedScorer{newScorer(t, 1, nil, nil)})
		require.NoError(t, err)

		require.NoError(t, e.StoreExtraction(t.Context(), closedSession()))
		// two participants get +1/2 each; winner (u2) also gets -1 -> net -0.5
		require.InDelta(t, 0.5, got["u1"], 1e-9)
		require.InDelta(t, -0.5, got["u2"], 1e-9)
	})

	t.Run("store error propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storemock.NewMockTransactionalStorage(ctrl)

		store.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context, persistence.Storage) error) error {
				return fn(ctx, store)
			},
		)
		store.EXPECT().IncreaseUserScore(gomock.Any(), gomock.Any()).
			Return(persistence.IncreaseUserScoreRet{}, errors.New("boom"))

		e, err := New(store, []WeightedScorer{newScorer(t, 1, nil, nil)})
		require.NoError(t, err)

		require.Error(t, e.StoreExtraction(t.Context(), closedSession()))
	})
}
