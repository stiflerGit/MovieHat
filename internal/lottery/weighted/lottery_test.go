package weighted

import (
	"context"
	"errors"
	"testing"

	"github.com/stiflerGit/moviehat/internal/lottery"
	"github.com/stiflerGit/moviehat/internal/lottery/weighted/scorer/fair_share/persistence"
	storemock "github.com/stiflerGit/moviehat/internal/lottery/weighted/scorer/fair_share/persistence/mocks"
	"github.com/stiflerGit/moviehat/internal/lottery/weighted/scorer/mocks"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// newScorer builds a WeightedScorer whose Scores() returns the given values.
func newScorer(t *testing.T, weight float64, scores []float64, err error) WeightedScorer {
	ctrl := gomock.NewController(t)
	m := mocks.NewMockInterface(ctrl)
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

func TestLottery_GetProbabilities(t *testing.T) {
	testCases := []struct {
		name    string
		userIDs []string
		scorer  func(t *testing.T) WeightedScorer
		want    []float64
		wantErr bool
	}{
		{
			name:    "no userIDs",
			userIDs: nil,
			scorer:  func(t *testing.T) WeightedScorer { return newScorer(t, 1, nil, nil) },
			want:    nil,
		},
		{
			name:    "single user is certain",
			userIDs: []string{"u1"},
			scorer:  func(t *testing.T) WeightedScorer { return newScorer(t, 1, nil, nil) },
			want:    []float64{1.0},
		},
		{
			name:    "scorer error propagates",
			userIDs: []string{"u1", "u2"},
			scorer:  func(t *testing.T) WeightedScorer { return newScorer(t, 1, nil, errors.New("boom")) },
			wantErr: true,
		},
		{
			name:    "zero scores fall back to uniform",
			userIDs: []string{"u1", "u2"},
			scorer:  func(t *testing.T) WeightedScorer { return newScorer(t, 1, []float64{0, 0}, nil) },
			want:    []float64{0.5, 0.5},
		},
		{
			name:    "non-zero scores are normalized",
			userIDs: []string{"u1", "u2", "u3"},
			scorer:  func(t *testing.T) WeightedScorer { return newScorer(t, 1, []float64{2, 0, 2}, nil) },
			// Normalize adds |min| (0) then divides by sum(4): 0.5, 0, 0.5
			want: []float64{0.5, 0, 0.5},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e, err := New(nil, []WeightedScorer{tc.scorer(t)})
			require.NoError(t, err)

			ret, err := e.GetProbabilities(t.Context(), lottery.GetProbabilitiesArg{UserIDs: tc.userIDs})
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.InDeltaSlice(t, tc.want, ret.Probabilities, 1e-9)
		})
	}
}

func TestLottery_GetProbabilities_CombinesWeightedScorers(t *testing.T) {
	// weights [3,1] normalize (shift by |min|) to [4/6, 2/6].
	// scorer A: all mass on u1 -> [1, 0]
	// scorer B: uniform fallback -> [0.5, 0.5]
	// expected: [4/6*1 + 2/6*0.5, 4/6*0 + 2/6*0.5] = [5/6, 1/6]
	e, err := New(nil, []WeightedScorer{
		newScorer(t, 3, []float64{1, 0}, nil),
		newScorer(t, 1, []float64{0, 0}, nil),
	})
	require.NoError(t, err)

	ret, err := e.GetProbabilities(t.Context(), lottery.GetProbabilitiesArg{UserIDs: []string{"u1", "u2"}})
	require.NoError(t, err)
	require.InDeltaSlice(t, []float64{5.0 / 6.0, 1.0 / 6.0}, ret.Probabilities, 1e-9)
}

func TestLottery_Draw(t *testing.T) {
	t.Run("single user wins", func(t *testing.T) {
		e, err := New(nil, []WeightedScorer{newScorer(t, 1, nil, nil)})
		require.NoError(t, err)

		ret, err := e.Draw(t.Context(), lottery.DrawArg{UserIDs: []string{"only"}})
		require.NoError(t, err)
		require.Equal(t, "only", ret.UserID)
		require.Equal(t, 0, ret.Index)
	})

	t.Run("never picks a zero-probability user and never panics", func(t *testing.T) {
		// u2 has probability 0; running many draws must never pick u2 nor panic.
		e, err := New(nil, []WeightedScorer{newScorer(t, 1, []float64{1, 0}, nil)})
		require.NoError(t, err)

		for range 1000 {
			ret, err := e.Draw(t.Context(), lottery.DrawArg{UserIDs: []string{"u1", "u2"}})
			require.NoError(t, err)
			require.Equal(t, "u1", ret.UserID)
			require.Equal(t, 0, ret.Index)
		}
	})

	t.Run("returns a real user and a consistent index across many draws", func(t *testing.T) {
		userIDs := []string{"u1", "u2", "u3"}
		e, err := New(nil, []WeightedScorer{newScorer(t, 1, []float64{1, 1, 1}, nil)})
		require.NoError(t, err)

		for range 1000 {
			ret, err := e.Draw(t.Context(), lottery.DrawArg{UserIDs: userIDs})
			require.NoError(t, err)
			require.GreaterOrEqual(t, ret.Index, 0)
			require.Less(t, ret.Index, len(userIDs))
			require.Equal(t, userIDs[ret.Index], ret.UserID)
		}
	})

	t.Run("empty userIDs returns error", func(t *testing.T) {
		// The fallback path indexes userIDs[len-1]; on an empty draw it must
		// return an error instead of panicking with a -1 index.
		e, err := New(nil, []WeightedScorer{newScorer(t, 1, nil, nil)})
		require.NoError(t, err)

		var drawErr error
		require.NotPanics(t, func() {
			_, drawErr = e.Draw(t.Context(), lottery.DrawArg{UserIDs: []string{}})
		})
		require.Error(t, drawErr)
	})
}

func TestLottery_StoreDrawResult(t *testing.T) {
	t.Run("empty winner returns error", func(t *testing.T) {
		e, err := New(nil, []WeightedScorer{newScorer(t, 1, nil, nil)})
		require.NoError(t, err)

		_, err = e.StoreDrawResult(t.Context(), lottery.StoreDrawResultArg{UserIDs: []string{"u1", "u2"}, WinnerID: ""})
		require.Error(t, err)
	})

	t.Run("empty participants returns error", func(t *testing.T) {
		e, err := New(nil, []WeightedScorer{newScorer(t, 1, nil, nil)})
		require.NoError(t, err)

		_, err = e.StoreDrawResult(t.Context(), lottery.StoreDrawResultArg{UserIDs: nil, WinnerID: "u1"})
		require.Error(t, err)
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

		_, err = e.StoreDrawResult(t.Context(), lottery.StoreDrawResultArg{UserIDs: []string{"u1", "u2"}, WinnerID: "u2"})
		require.NoError(t, err)
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

		_, err = e.StoreDrawResult(t.Context(), lottery.StoreDrawResultArg{UserIDs: []string{"u1", "u2"}, WinnerID: "u2"})
		require.Error(t, err)
	})
}
