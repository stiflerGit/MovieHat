package list

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stiflerGit/moviehat/internal/persistence"
	persistencemock "github.com/stiflerGit/moviehat/internal/persistence/mocks"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNew(t *testing.T) {
	h := New(nil)
	require.NotNil(t, h)
	require.NotNil(t, h.logger)
}

func TestHandlerAddUserMovie(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := persistencemock.NewMockTransactionalStorage(ctrl)

	h := New(store, WithLogger(testLogger()))

	store.EXPECT().AddMovie(gomock.Any(), persistence.AddMovieArg{UserID: "user-1", MovieID: "12345"}).
		Return(persistence.Movie{ID: "movie-1"}, nil)

	resp, err := h.AddUserMovie(t.Context(), AddUserMovieRequest{UserID: "user-1", MovieID: "12345"})
	require.NoError(t, err)
	require.Equal(t, "movie-1", resp.Movie.Id)
}

func TestHandlerDeleteUserMovie(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := persistencemock.NewMockTransactionalStorage(ctrl)

	h := New(store, WithLogger(testLogger()))

	store.EXPECT().DeleteMovie(gomock.Any(), persistence.DeleteMovieArg{UserID: "user-1", MovieID: "12345"}).
		Return(persistence.Movie{ID: "movie-1"}, nil)

	resp, err := h.DeleteUserMovie(t.Context(), DeleteUserMovieRequest{UserID: "user-1", MovieID: "12345"})
	require.NoError(t, err)
	require.NotNil(t, resp)
}
