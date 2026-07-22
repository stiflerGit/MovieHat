package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stiflerGit/moviehat/internal/extractor/fair_share/persistence"
	appmigrations "github.com/stiflerGit/moviehat/internal/migrations"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func newExtractorTestRepository(t *testing.T) *Storage {
	t.Helper()

	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, appmigrations.FS, goose.WithDisableVersioning(true))
	require.NoError(t, err)

	_, err = provider.Up(t.Context())
	require.NoError(t, err)

	return New(db)
}

func TestRepositoryIncreaseUserScore(t *testing.T) {
	r := newExtractorTestRepository(t)
	ctx := context.Background()

	ret, err := r.IncreaseUserScore(ctx, persistence.IncreaseUserScoreArg{UserID: "u1", Inc: 0.3})
	require.NoError(t, err)
	require.InDelta(t, 0.3, ret.UserScore.Score, 0.0001)

	ret, err = r.IncreaseUserScore(ctx, persistence.IncreaseUserScoreArg{UserID: "u1", Inc: 0.7})
	require.NoError(t, err)
	require.InDelta(t, 1.0, ret.UserScore.Score, 0.0001)
}

func TestRepositoryListUsersScores(t *testing.T) {
	r := newExtractorTestRepository(t)
	ctx := context.Background()

	_, _ = r.IncreaseUserScore(ctx, persistence.IncreaseUserScoreArg{UserID: "u1", Inc: 1})
	_, _ = r.IncreaseUserScore(ctx, persistence.IncreaseUserScoreArg{UserID: "u2", Inc: 2})

	all, err := r.ListUsersScores(ctx, persistence.ListUsersScoresArg{})
	require.NoError(t, err)
	require.Len(t, all.UserScores, 2)

	filtered, err := r.ListUsersScores(ctx, persistence.ListUsersScoresArg{UserIDs: []string{"u2"}})
	require.NoError(t, err)
	require.Len(t, filtered.UserScores, 1)
	require.Equal(t, "u2", filtered.UserScores[0].UserID)
}

func TestRepositoryWithTx(t *testing.T) {
	r := newExtractorTestRepository(t)
	ctx := context.Background()

	err := r.WithTx(ctx, func(ctx context.Context, repo persistence.Repository) error {
		_, err := repo.IncreaseUserScore(ctx, persistence.IncreaseUserScoreArg{UserID: "u1", Inc: 1})
		return err
	})
	require.NoError(t, err)

	list, err := r.ListUsersScores(ctx, persistence.ListUsersScoresArg{UserIDs: []string{"u1"}})
	require.NoError(t, err)
	require.Len(t, list.UserScores, 1)

	err = r.WithTx(ctx, func(ctx context.Context, repo persistence.Repository) error {
		_, err := repo.IncreaseUserScore(ctx, persistence.IncreaseUserScoreArg{UserID: "u2", Inc: 1})
		require.NoError(t, err)
		return errors.New("boom")
	})
	require.Error(t, err)

	list, err = r.ListUsersScores(ctx, persistence.ListUsersScoresArg{UserIDs: []string{"u2"}})
	require.NoError(t, err)
	require.Empty(t, list.UserScores)
}
