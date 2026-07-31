package extractor

import (
	"errors"
	"testing"

	"github.com/stiflerGit/moviehat/internal/extractor/weighted/scorer/fair_share/persistence"
	storemocks "github.com/stiflerGit/moviehat/internal/extractor/weighted/scorer/fair_share/persistence/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProvider_Probabilities(t *testing.T) {
	tests := []struct {
		name          string
		userIDs       []string
		initStoreMock func(t *storemocks.MockTransactionalStorage)
		want          []float64
		wantErr       bool
	}{
		{name: "nil current returns nil"},
		{name: "empty participants returns nil winner", userIDs: []string{}},
		{
			name:    "empty scores is treated as zero scores",
			userIDs: []string{"u-1"},
			initStoreMock: func(store *storemocks.MockTransactionalStorage) {
				store.EXPECT().ListUsersScores(gomock.Any(), persistence.ListUsersScoresArg{UserIDs: []string{"u-1"}}).
					Return(persistence.ListUsersScoresRet{}, nil)
			},
			want: []float64{0},
		},
		{
			name:    "single user with zero score",
			userIDs: []string{"u-1"},
			initStoreMock: func(store *storemocks.MockTransactionalStorage) {
				store.EXPECT().ListUsersScores(gomock.Any(), persistence.ListUsersScoresArg{UserIDs: []string{"u-1"}}).
					Return(persistence.ListUsersScoresRet{UserScores: []persistence.UserScore{{UserID: "u-1", Score: 0}}}, nil)
			},
			want: []float64{0},
		},
		{
			name:    "single participant with positive score",
			userIDs: []string{"u-1"},
			initStoreMock: func(store *storemocks.MockTransactionalStorage) {
				store.EXPECT().ListUsersScores(gomock.Any(), persistence.ListUsersScoresArg{UserIDs: []string{"u-1"}}).
					Return(persistence.ListUsersScoresRet{UserScores: []persistence.UserScore{{UserID: "u-1", Score: 0.5}}}, nil)
			},
			want: []float64{0.5},
		},
		{
			name:    "single participant with negative score",
			userIDs: []string{"u-1"},
			initStoreMock: func(store *storemocks.MockTransactionalStorage) {
				store.EXPECT().ListUsersScores(gomock.Any(), persistence.ListUsersScoresArg{UserIDs: []string{"u-1"}}).
					Return(persistence.ListUsersScoresRet{UserScores: []persistence.UserScore{{UserID: "u-1", Score: -0.5}}}, nil)
			},
			want: []float64{0},
		},
		{
			name:    "2 users with same score",
			userIDs: []string{"u-1", "u-2"},
			initStoreMock: func(store *storemocks.MockTransactionalStorage) {
				store.EXPECT().ListUsersScores(gomock.Any(), persistence.ListUsersScoresArg{UserIDs: []string{"u-1", "u-2"}}).
					Return(persistence.ListUsersScoresRet{UserScores: []persistence.UserScore{{UserID: "u-1", Score: 1}, {UserID: "u-2", Score: 1}}}, nil)
			},
			want: []float64{1, 1},
		},
		{
			name:    "4 users with same score",
			userIDs: []string{"u-1", "u-2", "u-3", "u-4"},
			initStoreMock: func(store *storemocks.MockTransactionalStorage) {
				store.EXPECT().ListUsersScores(gomock.Any(), persistence.ListUsersScoresArg{UserIDs: []string{"u-1", "u-2", "u-3", "u-4"}}).
					Return(persistence.ListUsersScoresRet{UserScores: []persistence.UserScore{{UserID: "u-1", Score: 1}, {UserID: "u-2", Score: 1}, {UserID: "u-3", Score: 1}, {UserID: "u-4", Score: 1}}}, nil)
			},
			want: []float64{1, 1, 1, 1},
		},
		{
			name:    "3 users with mixed positive scores",
			userIDs: []string{"u-1", "u-2", "u-3"},
			initStoreMock: func(store *storemocks.MockTransactionalStorage) {
				store.EXPECT().ListUsersScores(gomock.Any(), persistence.ListUsersScoresArg{UserIDs: []string{"u-1", "u-2", "u-3"}}).
					Return(persistence.ListUsersScoresRet{UserScores: []persistence.UserScore{{UserID: "u-1", Score: 1}, {UserID: "u-2", Score: 2}, {UserID: "u-3", Score: 3}}}, nil)
			},
			want: []float64{1, 2, 3},
		},
		{
			name:    "3 users with mixed scores",
			userIDs: []string{"u-1", "u-2", "u-3"},
			initStoreMock: func(store *storemocks.MockTransactionalStorage) {
				store.EXPECT().ListUsersScores(gomock.Any(), persistence.ListUsersScoresArg{UserIDs: []string{"u-1", "u-2", "u-3"}}).
					Return(persistence.ListUsersScoresRet{UserScores: []persistence.UserScore{{UserID: "u-1", Score: -1.0}, {UserID: "u-2", Score: 2}, {UserID: "u-3", Score: 3}}}, nil)
			},
			want: []float64{0, 2, 3},
		},
		{
			name:    "4 users with mixed scores - missing score at DB",
			userIDs: []string{"u-1", "u-2", "u-3", "u-4"},
			initStoreMock: func(store *storemocks.MockTransactionalStorage) {
				store.EXPECT().ListUsersScores(gomock.Any(), persistence.ListUsersScoresArg{UserIDs: []string{"u-1", "u-2", "u-3", "u-4"}}).
					Return(persistence.ListUsersScoresRet{UserScores: []persistence.UserScore{{UserID: "u-2", Score: -2}, {UserID: "u-3", Score: 3}, {UserID: "u-4", Score: 4}}}, nil)
			},
			want: []float64{0, 0, 3, 4},
		},
		{
			name:    "list users scores failure",
			userIDs: []string{"u-1"},
			initStoreMock: func(store *storemocks.MockTransactionalStorage) {
				store.EXPECT().ListUsersScores(gomock.Any(), persistence.ListUsersScoresArg{UserIDs: []string{"u-1"}}).
					Return(persistence.ListUsersScoresRet{}, errors.New("boom-list"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := storemocks.NewMockTransactionalStorage(gomock.NewController(t))
			if tt.initStoreMock != nil {
				tt.initStoreMock(store)
			}

			p := New(store)

			got, err := p.Scores(t.Context(), tt.userIDs)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			assert.InDeltaSlice(t, tt.want, got, 0.01)
		})
	}
}
