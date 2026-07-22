package extractor

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	repository "github.com/stiflerGit/moviehat/internal/extractor/fair_share/persistence"
	storemocks "github.com/stiflerGit/moviehat/internal/extractor/fair_share/persistence/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func newUser(id string) *pb.User {
	return &pb.User{Id: id, Name: id}
}

func newSession(participants []string, winner string) *pb.Session {
	users := make([]*pb.User, 0, len(participants))
	for _, p := range participants {
		users = append(users, newUser(p))
	}
	return &pb.Session{Participants: users, Winner: newUser(winner)}
}

func TestExtractor_Extract(t *testing.T) {
	tests := []struct {
		name         string
		current      *pb.Session
		ret          repository.ListUsersScoresRet
		listErr      error
		wantWinnerID string
		wantErr      bool
		wantListCall bool
	}{
		{name: "nil current returns nil winner"},
		{name: "empty participants returns nil winner", current: &pb.Session{}},
		{
			name:         "single participant is selected",
			current:      &pb.Session{Participants: []*pb.User{newUser("u-1")}},
			ret:          repository.ListUsersScoresRet{UserScores: []repository.UserScore{{UserID: "u-1", Score: 0.5}}},
			wantWinnerID: "u-1",
			wantListCall: true,
		},
		{
			name:         "empty scores is treated as zero scores",
			current:      &pb.Session{Participants: []*pb.User{newUser("u-1")}},
			ret:          repository.ListUsersScoresRet{},
			wantWinnerID: "u-1",
			wantListCall: true,
		},
		{
			name:         "list users scores failure",
			current:      &pb.Session{Participants: []*pb.User{newUser("u-1")}},
			listErr:      errors.New("boom-list"),
			wantErr:      true,
			wantListCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := storemocks.NewMockStore(ctrl)
			if tt.wantListCall {
				store.EXPECT().
					ListUsersScores(gomock.Any(), repository.ListUsersScoresArg{UserIDs: []string{"u-1"}}).
					Return(tt.ret, tt.listErr)
			}

			e := New(store, WithEqualChanceRate(defaultEqualChanceRate))
			got, err := e.Extract(t.Context(), tt.current)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, got)
				assert.ErrorContains(t, err, "e.fetchUserScoresFromRepository")
				return
			}

			require.NoError(t, err)
			if tt.wantWinnerID == "" {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.wantWinnerID, got.Id)
		})
	}
}

func TestExtractor_StoreExtraction(t *testing.T) {
	t.Run("stores participant shares and winner penalty", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storemocks.NewMockStore(ctrl)
		session := &pb.Session{
			Participants: []*pb.User{newUser("u-1"), newUser("u-2")},
			Winner:       newUser("u-2"),
			ClosedAt:     timestamppb.Now(),
		}

		store.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context, repository.Repository) error) error {
				return fn(ctx, store)
			},
		)
		store.EXPECT().IncreaseUserScore(gomock.Any(), repository.IncreaseUserScoreArg{UserID: "u-1", Inc: 0.5}).Return(repository.IncreaseUserScoreRet{}, nil)
		store.EXPECT().IncreaseUserScore(gomock.Any(), repository.IncreaseUserScoreArg{UserID: "u-2", Inc: 0.5}).Return(repository.IncreaseUserScoreRet{}, nil)
		store.EXPECT().IncreaseUserScore(gomock.Any(), repository.IncreaseUserScoreArg{UserID: "u-2", Inc: -1.0}).Return(repository.IncreaseUserScoreRet{}, nil)

		err := New(store).StoreExtraction(t.Context(), session)
		require.NoError(t, err)
	})

	t.Run("requires winner", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storemocks.NewMockStore(ctrl)
		err := New(store).StoreExtraction(t.Context(), &pb.Session{ClosedAt: timestamppb.Now()})
		require.Error(t, err)
		assert.ErrorContains(t, err, "winner is empty")
	})

	t.Run("requires valid closed at", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storemocks.NewMockStore(ctrl)
		err := New(store).StoreExtraction(t.Context(), &pb.Session{Winner: newUser("u-1")})
		require.Error(t, err)
		assert.ErrorContains(t, err, "closedAt is invalid")
	})

	t.Run("propagates increase failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storemocks.NewMockStore(ctrl)
		session := &pb.Session{Participants: []*pb.User{newUser("u-1")}, Winner: newUser("u-1"), ClosedAt: timestamppb.Now()}

		store.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context, repository.Repository) error) error {
				return fn(ctx, store)
			},
		)
		store.EXPECT().IncreaseUserScore(gomock.Any(), repository.IncreaseUserScoreArg{UserID: "u-1", Inc: 1.0}).Return(repository.IncreaseUserScoreRet{}, errors.New("boom-increase"))

		err := New(store).StoreExtraction(t.Context(), session)
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom-increase")
	})
}

func TestExtractor_ExtractWithHistory_Deterministic(t *testing.T) {
	e := New(nil, WithEqualChanceRate(defaultEqualChanceRate))

	got, err := e.ExtractWithHistory(t.Context(), nil, nil)
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestExtractor_extract_SkipsZeroProbabilityParticipants(t *testing.T) {
	e := New(nil, WithEqualChanceRate(0))
	participants := []*pb.User{newUser("a"), newUser("b"), newUser("c")}

	got := e.extract(map[string]float64{
		"a": -0.5,
		"b": 0.5,
	}, participants)

	require.Equal(t, "b", participants[got].Id)
}

func TestExtractor_ExtractWithHistory_Probability(t *testing.T) {
	t.Parallel()

	tests := []probabilityTestCase{
		{
			name:    "single participant always wins",
			history: nil,
			current: &pb.Session{Participants: []*pb.User{{Id: "a", Name: "a"}}},
			want:    map[string]float64{"a": 1},
		},
		{
			name:    "no history uniform distribution",
			history: nil,
			current: newSession([]string{"a", "b", "c", "d"}, ""),
			want: map[string]float64{
				"a": 0.25,
				"b": 0.25,
				"c": 0.25,
				"d": 0.25,
			},
		},
		{
			name: "all negative scores fallback to uniform",
			history: []*pb.Session{
				newSession([]string{"a", "fa1", "fa2", "fa3"}, "a"),
				newSession([]string{"b", "fb1", "fb2", "fb3"}, "b"),
			},
			current: newSession([]string{"a", "b"}, ""),
			want: map[string]float64{
				"a": 0.5,
				"b": 0.5,
			},
		},
		{
			name:    "pure fairness only one positive score always wins",
			history: []*pb.Session{newSession([]string{"a", "b"}, "b")},
			current: newSession([]string{"a", "b"}, ""),
			want:    map[string]float64{"a": 1.0},
		},
		{
			name:            "pure fairness equal positive scores even distribution",
			equalChanceRate: 0,
			history:         []*pb.Session{newSession([]string{"a", "b", "c"}, "c")},
			current:         newSession([]string{"a", "b"}, ""),
			want:            map[string]float64{"a": .5, "b": .5},
		},
		{
			name: "single positive chance maps to later participant",
			history: []*pb.Session{
				newSession([]string{"a", "b", "c"}, "b"),
				newSession([]string{"a", "b"}, "a"),
			},
			current: newSession([]string{"a", "b", "c"}, ""),
			want:    map[string]float64{"c": 1.0},
		},
		{
			name: "score update proportional to session size",
			history: []*pb.Session{
				newSession([]string{"a", "fa1"}, "fa"),
				newSession([]string{"b", "fb1", "fb2", "fb3", "fb4", "fb5", "fb6", "fb7", "fb8", "fb9"}, "fb1"),
			},
			current: newSession([]string{"a", "b"}, ""),
			want: map[string]float64{
				"a": 0.5 / 0.6,
				"b": 0.1 / 0.6,
			},
		},
		{
			name:            "spec example session 2",
			equalChanceRate: 0.3,
			history:         []*pb.Session{newSession([]string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}, "a")},
			current:         newSession([]string{"a", "b", "c"}, ""),
			want:            map[string]float64{"a": 0.1, "b": 0.45, "c": 0.45},
		},
		{
			name:            "spec example session 4",
			equalChanceRate: 0.3,
			history: []*pb.Session{
				newSession([]string{"a", "b", "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8"}, "f1"),
				newSession([]string{"a", "b", "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8"}, "f2"),
				newSession([]string{"a", "b", "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8"}, "f3"),
			},
			current: newSession([]string{"a", "b", "c", "d"}, ""),
			want:    map[string]float64{"a": 0.425, "b": 0.425, "c": 0.075, "d": 0.075},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testProbability(t, tt)
		})
	}
}

type probabilityTestCase struct {
	name            string
	equalChanceRate float64
	history         []*pb.Session
	current         *pb.Session
	want            map[string]float64
}

func testProbability(t *testing.T, tc probabilityTestCase) {
	e := New(nil, WithEqualChanceRate(tc.equalChanceRate))

	count := map[string]float64{}
	sum := 0
	for range 10_000 {
		got, err := e.ExtractWithHistory(t.Context(), tc.history, tc.current)
		require.NoError(t, err)
		count[got.Id]++
		sum++
	}

	require.ElementsMatch(t, slices.Collect(maps.Keys(tc.want)), slices.Collect(maps.Keys(count)))

	for wantK, wantV := range tc.want {
		gotV := count[wantK] / float64(sum)
		assert.InDelta(t, wantV, gotV, 0.015)
	}
}
