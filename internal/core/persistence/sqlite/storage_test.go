package sqlite

import (
	"database/sql"
	"errors"
	"testing"

	appmigrations "github.com/stiflerGit/moviehat/internal/migrations"
	"github.com/stiflerGit/moviehat/internal/core/persistence"
	"github.com/stiflerGit/moviehat/pkg/sql/tx"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func newMovieHatTestStorage(t *testing.T) *Storage {
	t.Helper()

	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, appmigrations.FS, goose.WithDisableVersioning(true))
	require.NoError(t, err)

	_, err = provider.Up(t.Context())
	require.NoError(t, err)

	return New(tx.NewManager(db))
}

func TestStorageWithTx(t *testing.T) {
	s := newMovieHatTestStorage(t)

	err := s.WithTx(t.Context(), nil)
	require.Error(t, err)
	var invalidArg persistence.ErrInvalidArgument
	require.ErrorAs(t, err, &invalidArg)
}

func TestStorageUserLifecycle(t *testing.T) {
	s := newMovieHatTestStorage(t)
	ctx := t.Context()
	authUserID := uuid.NewString()

	_, err := s.CreateUser(ctx, persistence.CreateUserArg{})
	require.Error(t, err)

	user, err := s.CreateUser(ctx, persistence.CreateUserArg{UserID: authUserID})
	require.NoError(t, err)
	require.Equal(t, authUserID, user.ID)

	got, err := s.GetUser(ctx, persistence.GetUserArg{UserID: authUserID})
	require.NoError(t, err)
	require.Equal(t, user.ID, got.ID)

	users, err := s.ListUsers(ctx, persistence.ListUsersArg{})
	require.NoError(t, err)
	require.Len(t, users, 1)

	updated, err := s.UpdateUser(ctx, persistence.UpdateUserArg{UserID: user.ID, Name: "john"})
	require.NoError(t, err)
	require.Equal(t, "john", updated.Name)

	deleted, err := s.DeleteUser(ctx, persistence.DeleteUserArg{UserID: user.ID})
	require.NoError(t, err)
	require.False(t, deleted.DeletedAt.IsZero())
}

func TestStorageSessionLifecycle(t *testing.T) {
	s := newMovieHatTestStorage(t)
	ctx := t.Context()

	created, err := s.CreateSession(ctx, persistence.CreateSessionArg{})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)

	_, err = s.CreateSession(ctx, persistence.CreateSessionArg{})
	require.ErrorIs(t, err, persistence.ErrAlreadyExists)

	list, err := s.ListSessions(ctx, persistence.ListSessionsAg{})
	require.NoError(t, err)
	require.Len(t, list.Sessions, 1)

	err = s.CloseSession(ctx, created.ID)
	require.NoError(t, err)

	err = s.CloseSession(ctx, created.ID)
	require.ErrorIs(t, err, persistence.ErrSessionClosed)

	got, err := s.GetSession(ctx, persistence.GetSessionArg{ID: created.ID})
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)

	list, err = s.ListSessions(ctx, persistence.ListSessionsAg{})
	require.NoError(t, err)
	require.Len(t, list.Sessions, 1)

	deleted, err := s.DeleteSession(ctx, created.ID)
	require.NoError(t, err)
	require.False(t, deleted.DeletedAt.IsZero())

	list, err = s.ListSessions(ctx, persistence.ListSessionsAg{})
	require.NoError(t, err)
	require.Len(t, list.Sessions, 0)
}

func TestStorageParticipantsLifecycle(t *testing.T) {
	s := newMovieHatTestStorage(t)
	ctx := t.Context()

	authUserID := uuid.NewString()
	user, err := s.CreateUser(ctx, persistence.CreateUserArg{UserID: authUserID})
	require.NoError(t, err)
	_, err = s.UpdateUser(ctx, persistence.UpdateUserArg{UserID: user.ID, Name: "john"})
	require.NoError(t, err)

	session, err := s.CreateSession(ctx, persistence.CreateSessionArg{})
	require.NoError(t, err)

	participant, err := s.CreateParticipant(ctx, persistence.CreateParticipantArg{SessionID: session.ID, UserID: user.ID})
	require.NoError(t, err)
	require.Equal(t, session.ID, participant.SessionID)

	// create participant again to test idempotency
	participant, err = s.CreateParticipant(ctx, persistence.CreateParticipantArg{SessionID: session.ID, UserID: user.ID})
	require.Error(t, err)
	require.ErrorIs(t, err, persistence.ErrAlreadyExists)

	list, err := s.ListParticipants(ctx, persistence.ListParticipantsArg{SessionID: session.ID})
	require.NoError(t, err)
	require.Len(t, list.Participants, 1)
	require.Equal(t, "john", list.Participants[0].Name)

	err = s.DeleteParticipant(ctx, persistence.DeleteParticipantArg{SessionID: session.ID, UserID: user.ID})
	require.NoError(t, err)

	err = s.DeleteParticipant(ctx, persistence.DeleteParticipantArg{SessionID: session.ID, UserID: user.ID})
	require.ErrorIs(t, err, persistence.ErrNotFound)
}

func TestStorageMoviesLifecycle(t *testing.T) {
	s := newMovieHatTestStorage(t)
	ctx := t.Context()
	authUserID := uuid.NewString()

	user, err := s.CreateUser(ctx, persistence.CreateUserArg{UserID: authUserID})
	require.NoError(t, err)

	_, err = s.AddMovie(ctx, persistence.AddMovieArg{UserID: authUserID})
	require.Error(t, err)
	var invalidArg persistence.ErrInvalidArgument
	require.ErrorAs(t, err, &invalidArg)

	movie, err := s.AddMovie(ctx, persistence.AddMovieArg{UserID: authUserID, MovieID: "12345", Title: "Alien"})
	require.NoError(t, err)
	require.Equal(t, "Alien", movie.Title)
	require.Equal(t, "12345", movie.ID)

	list, err := s.GetMovieList(ctx, persistence.GetMovieListArg{UserID: user.ID})
	require.NoError(t, err)
	require.Len(t, list.Movies, 1)

	deleted, err := s.DeleteMovie(ctx, persistence.DeleteMovieArg{UserID: user.ID, MovieID: movie.ID})
	require.NoError(t, err)
	require.Equal(t, "Alien", deleted.Title)
	require.Equal(t, "12345", movie.ID)

	_, err = s.GetMovieList(ctx, persistence.GetMovieListArg{UserID: user.ID})
	require.Error(t, err)
	require.True(t, errors.Is(err, persistence.ErrNotFound))
}

func TestStorageUpdateSession(t *testing.T) {
	t.Run("updates watched_movie_id for a closed session", func(t *testing.T) {
		s := newMovieHatTestStorage(t)
		ctx := t.Context()

		userID := uuid.NewString()
		_, err := s.CreateUser(ctx, persistence.CreateUserArg{UserID: userID})
		require.NoError(t, err)

		session, err := s.CreateSession(ctx, persistence.CreateSessionArg{})
		require.NoError(t, err)
		require.NoError(t, s.CloseSession(ctx, session.ID))

		movie, err := s.AddMovie(ctx, persistence.AddMovieArg{UserID: userID, MovieID: "12345", Title: "Alien"})
		require.NoError(t, err)

		updated, err := s.UpdateSession(ctx, persistence.UpdateSessionArg{ID: session.ID, WatchedMovieID: &movie.ID})
		require.NoError(t, err)
		require.Equal(t, movie.ID, updated.WatchedMovieID)
	})

	t.Run("rejects invalid watched_movie_id", func(t *testing.T) {
		s := newMovieHatTestStorage(t)
		ctx := t.Context()

		session, err := s.CreateSession(ctx, persistence.CreateSessionArg{})
		require.NoError(t, err)
		require.NoError(t, s.CloseSession(ctx, session.ID))

		emptyMovieID := ""
		_, err = s.UpdateSession(ctx, persistence.UpdateSessionArg{ID: session.ID, WatchedMovieID: &emptyMovieID})
		require.Error(t, err)
		var invalidArg persistence.ErrInvalidArgument
		require.ErrorAs(t, err, &invalidArg)
	})
}

func TestStorage_UpdateMovies(t *testing.T) {
	s := newMovieHatTestStorage(t)
	ctx := t.Context()

	// user 1
	userID1 := uuid.NewString()
	_, err := s.CreateUser(ctx, persistence.CreateUserArg{UserID: userID1})
	require.NoError(t, err)

	movie11, err := s.AddMovie(ctx, persistence.AddMovieArg{UserID: userID1, MovieID: "mov1", Title: "Alien"})
	require.NoError(t, err)
	require.Equal(t, persistence.MovieStatusPending, movie11.Status)

	movie12, err := s.AddMovie(ctx, persistence.AddMovieArg{UserID: userID1, MovieID: "mov12", Title: "Alien 2"})
	require.NoError(t, err)
	require.Equal(t, persistence.MovieStatusPending, movie12.Status)

	// user 1
	userID2 := uuid.NewString()
	_, err = s.CreateUser(ctx, persistence.CreateUserArg{UserID: userID2})
	require.NoError(t, err)

	// same movie of user 1
	movie21, err := s.AddMovie(ctx, persistence.AddMovieArg{UserID: userID2, MovieID: "mov1", Title: "Alien"})
	require.NoError(t, err)
	require.Equal(t, persistence.MovieStatusPending, movie21.Status)

	statusWatched := persistence.MovieStatusWatched

	err = s.UpdateMovies(t.Context(), persistence.UpdateMoviesArg{ID: "mov1", Status: &statusWatched})
	require.NoError(t, err)

	m, err := s.GetMovie(ctx, persistence.GetMovieArg{UserID: userID1, MovieID: movie11.ID})
	require.NoError(t, err)
	assert.Equal(t, statusWatched, m.Status)

	m, err = s.GetMovie(ctx, persistence.GetMovieArg{UserID: userID1, MovieID: movie21.ID})
	require.NoError(t, err)
	assert.Equal(t, statusWatched, m.Status)
}
