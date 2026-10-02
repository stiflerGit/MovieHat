package user

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
	h := New(nil, nil, nil)
	require.NotNil(t, h)
	require.NotNil(t, h.logger)
}

func TestHandlerCreateUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := persistencemock.NewMockTransactionalStorage(ctrl)
	h := New(store, nil, WithLogger(testLogger()))

	store.EXPECT().CreateUser(gomock.Any(), persistence.CreateUserArg{UserID: "auth-u1"}).
		Return(persistence.User{ID: "auth-u1", Name: "john"}, nil)

	resp, err := h.CreateUser(t.Context(), CreateUserRequest{UserID: "auth-u1"})
	require.NoError(t, err)
	require.Equal(t, "auth-u1", resp.User.Id)
}
