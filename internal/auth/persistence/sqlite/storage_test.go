package sqlite

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/stiflerGit/moviehat/internal/auth/persistence"
	appmigrations "github.com/stiflerGit/moviehat/internal/migrations"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func newAuthTestStorage(t *testing.T) *Storage {
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

func TestStorageInsertUser(t *testing.T) {
	s := newAuthTestStorage(t)

	ret, err := s.InsertUser(t.Context(), persistence.InsertUserArg{
		Email:          "john@doe.com",
		HashedPassword: "hash",
	})
	require.NoError(t, err)
	require.NotEmpty(t, ret.User.ID)
	require.Equal(t, "john@doe.com", ret.User.Email)

	_, err = s.InsertUser(t.Context(), persistence.InsertUserArg{
		Email:          "john@doe.com",
		HashedPassword: "hash",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, persistence.ErrAlreadyExists))
}

func TestStorageGetUser(t *testing.T) {
	s := newAuthTestStorage(t)
	ctx := t.Context()

	_, err := s.GetUser(ctx, persistence.GetUserArg{})
	require.EqualError(t, err, "empty input")

	_, err = s.GetUser(ctx, persistence.GetUserArg{Email: "none@doe.com"})
	require.ErrorIs(t, err, persistence.ErrNotFound)

	insertRet, err := s.InsertUser(ctx, persistence.InsertUserArg{Email: "john@doe.com", HashedPassword: "hash"})
	require.NoError(t, err)

	byEmail, err := s.GetUser(ctx, persistence.GetUserArg{Email: "john@doe.com"})
	require.NoError(t, err)
	require.Equal(t, insertRet.User.ID, byEmail.User.ID)

	byID, err := s.GetUser(ctx, persistence.GetUserArg{ID: insertRet.User.ID})
	require.NoError(t, err)
	require.Equal(t, "john@doe.com", byID.User.Email)
}

func TestStorageSessionLifecycle(t *testing.T) {
	s := newAuthTestStorage(t)
	ctx := t.Context()

	insertUserRet, err := s.InsertUser(ctx, persistence.InsertUserArg{Email: "john@doe.com", HashedPassword: "hash"})
	require.NoError(t, err)

	expiresAt := time.Now().Add(time.Hour).Round(time.Second)
	insertSessionRet, err := s.InsertSession(ctx, persistence.InsertSessionArg{Token: "token-1", UserID: insertUserRet.User.ID, ExpiresAt: expiresAt})
	require.NoError(t, err)
	require.Equal(t, insertUserRet.User.ID, insertSessionRet.Session.UserID)

	_, err = s.InsertSession(ctx, persistence.InsertSessionArg{Token: "token-1", UserID: insertUserRet.User.ID, ExpiresAt: expiresAt})
	require.Error(t, err)
	require.True(t, errors.Is(err, persistence.ErrAlreadyExists))

	getRet, err := s.GetSession(ctx, persistence.GetSessionArg{Token: "token-1"})
	require.NoError(t, err)
	require.Equal(t, insertSessionRet.Session.ID, getRet.Session.ID)

	_, err = s.GetSession(ctx, persistence.GetSessionArg{})
	require.EqualError(t, err, "empty input")

	lastAccess := time.Now().Round(time.Second)
	newExpiresAt := time.Now().Add(2 * time.Hour).Round(time.Second)
	updateRet, err := s.UpdateSession(ctx, persistence.UpdateSessionArg{ID: insertSessionRet.Session.ID, LastAccess: &lastAccess, ExpiresAt: &newExpiresAt})
	require.NoError(t, err)
	require.WithinDuration(t, lastAccess, updateRet.Session.LastAccess, time.Second)
	require.WithinDuration(t, newExpiresAt, updateRet.Session.ExpiresAt, time.Second)

	_, err = s.UpdateSession(ctx, persistence.UpdateSessionArg{})
	require.EqualError(t, err, "empty input")
}

func TestStorageVerificationLifecycle(t *testing.T) {
	s := newAuthTestStorage(t)
	ctx := t.Context()

	insertUserRet, err := s.InsertUser(ctx, persistence.InsertUserArg{Email: "inviter@example.com", HashedPassword: "hash"})
	require.NoError(t, err)

	expiresAt := time.Now().Add(time.Hour).Round(time.Second)
	insertRet, err := s.InsertVerification(ctx, persistence.InsertVerificationArg{
		TokenHash: "token-hash-1",
		ExpiresAt: expiresAt,
		MaxUses:   10,
		AuthorID:  insertUserRet.User.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "token-hash-1", insertRet.Verification.TokenHash)
	require.Equal(t, 10, insertRet.Verification.MaxUses)
	require.Equal(t, 0, insertRet.Verification.UsesCount)
	require.Equal(t, insertUserRet.User.ID, insertRet.Verification.AuthorID)
	require.WithinDuration(t, expiresAt, insertRet.Verification.ExpiresAt, time.Second)

	getRet, err := s.GetVerification(ctx, persistence.GetVerificationArg{TokenHash: "token-hash-1"})
	require.NoError(t, err)
	require.Equal(t, insertRet.Verification.TokenHash, getRet.Verification.TokenHash)
	require.Equal(t, insertRet.Verification.MaxUses, getRet.Verification.MaxUses)
	require.Equal(t, insertRet.Verification.UsesCount, getRet.Verification.UsesCount)
	require.Equal(t, insertRet.Verification.AuthorID, getRet.Verification.AuthorID)
	require.WithinDuration(t, expiresAt, getRet.Verification.ExpiresAt, time.Second)

	newExpiresAt := time.Now().Add(2 * time.Hour).Round(time.Second)
	updateRet, err := s.UpdateVerification(ctx, persistence.UpdateVerificationArg{
		TokenHash: "token-hash-1",
		ExpiresAt: &newExpiresAt,
	})
	require.NoError(t, err)
	require.Equal(t, "token-hash-1", updateRet.Verification.TokenHash)
	require.Equal(t, 0, updateRet.Verification.UsesCount)
	require.WithinDuration(t, newExpiresAt, updateRet.Verification.ExpiresAt, time.Second)

	getUpdatedRet, err := s.GetVerification(ctx, persistence.GetVerificationArg{TokenHash: "token-hash-1"})
	require.NoError(t, err)
	require.Equal(t, 0, getUpdatedRet.Verification.UsesCount)
	require.WithinDuration(t, newExpiresAt, getUpdatedRet.Verification.ExpiresAt, time.Second)
}

func TestStorageVerificationErrors(t *testing.T) {
	s := newAuthTestStorage(t)
	ctx := t.Context()

	_, err := s.GetVerification(ctx, persistence.GetVerificationArg{TokenHash: "missing-token-hash"})
	require.ErrorIs(t, err, persistence.ErrNotFound)

	newExpiresAt := time.Now().Add(time.Hour)
	_, err = s.UpdateVerification(ctx, persistence.UpdateVerificationArg{TokenHash: "missing-token-hash", ExpiresAt: &newExpiresAt})
	require.ErrorIs(t, err, persistence.ErrNotFound)

	_, err = s.UpdateVerification(ctx, persistence.UpdateVerificationArg{TokenHash: "missing-token-hash"})
	require.EqualError(t, err, "empty input")

	insertUserRet, err := s.InsertUser(ctx, persistence.InsertUserArg{Email: "inviter@example.com", HashedPassword: "hash"})
	require.NoError(t, err)
	_, err = s.InsertVerification(ctx, persistence.InsertVerificationArg{
		TokenHash: "token-hash-1",
		ExpiresAt: time.Now().Add(time.Hour),
		MaxUses:   10,
		AuthorID:  insertUserRet.User.ID,
	})
	require.NoError(t, err)

	_, err = s.InsertVerification(ctx, persistence.InsertVerificationArg{
		TokenHash: "token-hash-1",
		ExpiresAt: time.Now().Add(time.Hour),
		MaxUses:   10,
		AuthorID:  insertUserRet.User.ID,
	})
	require.Error(t, err)
}

func TestStorageConsumeVerification(t *testing.T) {
	s := newAuthTestStorage(t)
	ctx := t.Context()

	_, err := s.ConsumeVerification(ctx, persistence.ConsumeVerificationArg{})
	require.EqualError(t, err, "empty token hash")

	_, err = s.ConsumeVerification(ctx, persistence.ConsumeVerificationArg{TokenHash: "missing-token-hash"})
	require.ErrorIs(t, err, persistence.ErrNotFound)

	insertUserRet, err := s.InsertUser(ctx, persistence.InsertUserArg{Email: "inviter@example.com", HashedPassword: "hash"})
	require.NoError(t, err)

	_, err = s.InsertVerification(ctx, persistence.InsertVerificationArg{
		TokenHash: "valid-token-hash",
		ExpiresAt: time.Now().Add(time.Hour),
		MaxUses:   2,
		AuthorID:  insertUserRet.User.ID,
	})
	require.NoError(t, err)

	consumeRet, err := s.ConsumeVerification(ctx, persistence.ConsumeVerificationArg{TokenHash: "valid-token-hash"})
	require.NoError(t, err)
	require.Equal(t, 1, consumeRet.Verification.UsesCount)

	consumeRet, err = s.ConsumeVerification(ctx, persistence.ConsumeVerificationArg{TokenHash: "valid-token-hash"})
	require.NoError(t, err)
	require.Equal(t, 2, consumeRet.Verification.UsesCount)

	_, err = s.ConsumeVerification(ctx, persistence.ConsumeVerificationArg{TokenHash: "valid-token-hash"})
	require.ErrorIs(t, err, persistence.ErrNotFound)

	getValidRet, err := s.GetVerification(ctx, persistence.GetVerificationArg{TokenHash: "valid-token-hash"})
	require.NoError(t, err)
	require.Equal(t, 2, getValidRet.Verification.UsesCount)

	_, err = s.InsertVerification(ctx, persistence.InsertVerificationArg{
		TokenHash: "expired-token-hash",
		ExpiresAt: time.Now().Add(-time.Hour),
		MaxUses:   10,
		AuthorID:  insertUserRet.User.ID,
	})
	require.NoError(t, err)

	_, err = s.ConsumeVerification(ctx, persistence.ConsumeVerificationArg{TokenHash: "expired-token-hash"})
	require.ErrorIs(t, err, persistence.ErrNotFound)

	getExpiredRet, err := s.GetVerification(ctx, persistence.GetVerificationArg{TokenHash: "expired-token-hash"})
	require.NoError(t, err)
	require.Equal(t, 0, getExpiredRet.Verification.UsesCount)
}
