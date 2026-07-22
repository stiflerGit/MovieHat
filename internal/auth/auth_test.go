package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/auth/persistence"
	persistencemock "github.com/stiflerGit/moviehat/internal/auth/persistence/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

func TestNew(t *testing.T) {
	t.Run("empty secret", func(t *testing.T) {
		h, err := New(nil, "")
		require.Error(t, err)
		require.Nil(t, h)
		require.EqualError(t, err, "secret is empty")
	})

	t.Run("ok", func(t *testing.T) {
		h, err := New(nil, "secret")
		require.NoError(t, err)
		require.NotNil(t, h)
		require.Equal(t, "secret", h.secret)
	})
}

func newTestHandler(t *testing.T, store persistence.TransactionalStorage) *Handler {
	t.Helper()

	h, err := New(store, "secret")
	require.NoError(t, err)
	return h
}

func expectTx(store *persistencemock.MockTransactionalStorage) {
	store.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, persistence.Storage) error) error {
			return fn(ctx, store)
		},
	)
}

func TestHandlerSignUp(t *testing.T) {
	t.Run("invalid request", func(t *testing.T) {
		h := &Handler{secret: "secret"}
		_, err := h.SignUp(t.Context(), &pb.SignUpRequest{})
		require.Error(t, err)
		require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("missing invitation token", func(t *testing.T) {
		h := &Handler{secret: "secret"}
		var err error
		require.NotPanics(t, func() {
			_, err = h.SignUp(t.Context(), &pb.SignUpRequest{Email: "a@b.com", Password: "password1"})
		})
		require.Error(t, err)
		require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("invitation not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)

		expectTx(store)
		store.EXPECT().ConsumeVerification(gomock.Any(), gomock.Any()).Return(persistence.ConsumeVerificationRet{}, persistence.ErrNotFound)

		_, err := h.SignUp(t.Context(), &pb.SignUpRequest{Email: "a@b.com", Password: "password1", InvitationToken: "invite-token"})
		require.Error(t, err)
		require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	})

	t.Run("expired invitation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)

		expectTx(store)
		store.EXPECT().ConsumeVerification(gomock.Any(), gomock.Any()).Return(persistence.ConsumeVerificationRet{}, persistence.ErrNotFound)

		_, err := h.SignUp(t.Context(), &pb.SignUpRequest{Email: "a@b.com", Password: "password1", InvitationToken: "invite-token"})
		require.Error(t, err)
		require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	})

	t.Run("fully used invitation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)

		expectTx(store)
		store.EXPECT().ConsumeVerification(gomock.Any(), gomock.Any()).Return(persistence.ConsumeVerificationRet{}, persistence.ErrNotFound)

		_, err := h.SignUp(t.Context(), &pb.SignUpRequest{Email: "a@b.com", Password: "password1", InvitationToken: "invite-token"})
		require.Error(t, err)
		require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	})

	t.Run("already exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)

		expectTx(store)
		store.EXPECT().ConsumeVerification(gomock.Any(), gomock.Any()).Return(persistence.ConsumeVerificationRet{}, nil)
		store.EXPECT().
			InsertUser(gomock.Any(), gomock.Any()).
			Return(persistence.InsertUserRet{}, persistence.ErrAlreadyExists)

		_, err := h.SignUp(t.Context(), &pb.SignUpRequest{Email: "a@b.com", Password: "password1", InvitationToken: "invite-token"})
		require.Error(t, err)
		require.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)

		wantTokenHash, err := tokenHash("invite-token", "secret")
		require.NoError(t, err)

		expectTx(store)
		store.EXPECT().ConsumeVerification(gomock.Any(), persistence.ConsumeVerificationArg{TokenHash: wantTokenHash}).Return(persistence.ConsumeVerificationRet{}, nil)
		store.EXPECT().
			InsertUser(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, in persistence.InsertUserArg) (persistence.InsertUserRet, error) {
				require.Equal(t, "a@b.com", in.Email)
				require.NoError(t, bcrypt.CompareHashAndPassword([]byte(in.HashedPassword), []byte("password1")))
				return persistence.InsertUserRet{User: persistence.User{ID: "user-1"}}, nil
			})
		store.EXPECT().InsertSession(gomock.Any(), gomock.Any()).Return(persistence.InsertSessionRet{Session: persistence.Session{ID: "session-1"}}, nil)

		resp, err := h.SignUp(t.Context(), &pb.SignUpRequest{Email: "a@b.com", Password: "password1", InvitationToken: "invite-token"})
		require.NoError(t, err)
		require.Equal(t, "user-1", resp.UserID)
		require.NotEmpty(t, resp.Token)
	})
}

func TestHandlerSignIn(t *testing.T) {
	t.Run("invalid request", func(t *testing.T) {
		h := &Handler{secret: "secret"}
		_, err := h.SignIn(t.Context(), &pb.SignInRequest{})
		require.Error(t, err)
		require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("user not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)

		store.EXPECT().GetUser(gomock.Any(), persistence.GetUserArg{Email: "a@b.com"}).Return(persistence.GetUserRet{}, persistence.ErrNotFound)

		_, err := h.SignIn(t.Context(), &pb.SignInRequest{Email: "a@b.com", Password: "password1"})
		require.Error(t, err)
		require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("wrong password", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)

		hash, err := bcrypt.GenerateFromPassword([]byte("password1"), bcrypt.DefaultCost)
		require.NoError(t, err)
		store.EXPECT().GetUser(gomock.Any(), gomock.Any()).Return(
			persistence.GetUserRet{User: persistence.User{ID: "u1", HashedPassword: string(hash)}},
			nil,
		)

		_, err = h.SignIn(t.Context(), &pb.SignInRequest{Email: "a@b.com", Password: "another-password"})
		require.Error(t, err)
		require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)

		hash, err := bcrypt.GenerateFromPassword([]byte("password1"), bcrypt.DefaultCost)
		require.NoError(t, err)

		store.EXPECT().GetUser(gomock.Any(), persistence.GetUserArg{Email: "a@b.com"}).Return(
			persistence.GetUserRet{User: persistence.User{ID: "u1", HashedPassword: string(hash)}},
			nil,
		)

		var insertArg persistence.InsertSessionArg
		store.EXPECT().InsertSession(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, in persistence.InsertSessionArg) (persistence.InsertSessionRet, error) {
				insertArg = in
				return persistence.InsertSessionRet{Session: persistence.Session{ID: "s1"}}, nil
			},
		)

		resp, err := h.SignIn(t.Context(), &pb.SignInRequest{Email: "a@b.com", Password: "password1"})
		require.NoError(t, err)
		require.NotEmpty(t, resp.Token)
		require.Equal(t, "u1", insertArg.UserID)
		require.WithinDuration(t, time.Now().Add(defaultSessionDuration), insertArg.ExpiresAt, 2*time.Second)

		hashed, err := tokenHash(resp.Token, "secret")
		require.NoError(t, err)
		require.Equal(t, hashed, insertArg.Token)
	})
}

func TestHandlerSignOut(t *testing.T) {
	t.Run("missing session", func(t *testing.T) {
		h := &Handler{secret: "secret"}
		_, err := h.SignOut(t.Context(), &pb.SignOutRequest{})
		require.Error(t, err)
		require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)

		ctx := authn.SetInfo(t.Context(), Session{ID: "s1"})
		store.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, in persistence.UpdateSessionArg) (persistence.UpdateSessionRet, error) {
				require.Equal(t, "s1", in.ID)
				require.NotNil(t, in.LastAccess)
				require.NotNil(t, in.ExpiresAt)
				require.WithinDuration(t, *in.LastAccess, *in.ExpiresAt, 0)
				return persistence.UpdateSessionRet{}, nil
			},
		)

		resp, err := h.SignOut(ctx, &pb.SignOutRequest{})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestHandlerValidateSession(t *testing.T) {
	t.Run("empty token", func(t *testing.T) {
		h := &Handler{secret: "secret"}
		_, err := h.ValidateSession(t.Context(), &ValidateSessionRequest{})
		require.Error(t, err)
		require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("session not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)

		store.EXPECT().GetSession(gomock.Any(), gomock.Any()).Return(persistence.GetSessionRet{}, persistence.ErrNotFound)

		_, err := h.ValidateSession(t.Context(), &ValidateSessionRequest{Token: "token"})
		require.Error(t, err)
		require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("expired token", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)

		store.EXPECT().GetSession(gomock.Any(), gomock.Any()).Return(
			persistence.GetSessionRet{Session: persistence.Session{ID: "s1", ExpiresAt: time.Now().Add(-time.Minute)}},
			nil,
		)

		_, err := h.ValidateSession(t.Context(), &ValidateSessionRequest{Token: "token"})
		require.Error(t, err)
		require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)

		store.EXPECT().GetSession(gomock.Any(), gomock.Any()).Return(
			persistence.GetSessionRet{Session: persistence.Session{ID: "s1", Token: "t", UserID: "u1", CreatedAt: time.Unix(10, 0), ExpiresAt: time.Now().Add(time.Minute)}},
			nil,
		)

		store.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, in persistence.UpdateSessionArg) (persistence.UpdateSessionRet, error) {
				require.Equal(t, "s1", in.ID)
				require.NotNil(t, in.LastAccess)
				require.NotNil(t, in.ExpiresAt)
				return persistence.UpdateSessionRet{Session: persistence.Session{
					ID:         "s1",
					Token:      "t",
					UserID:     "u1",
					CreatedAt:  time.Unix(10, 0),
					ExpiresAt:  *in.ExpiresAt,
					LastAccess: *in.LastAccess,
				}}, nil
			},
		)

		resp, err := h.ValidateSession(t.Context(), &ValidateSessionRequest{Token: "token"})
		require.NoError(t, err)
		require.Equal(t, "u1", resp.Session.UserId)
		require.Equal(t, "s1", resp.Session.ID)
		require.False(t, resp.Session.LastAccess.IsZero())
	})
}

func TestHandlerDeleteUser(t *testing.T) {
	t.Run("missing session", func(t *testing.T) {
		h := &Handler{secret: "secret"}

		_, err := h.DeleteUser(t.Context(), &pb.DeleteUserRequest{})
		require.Error(t, err)
		require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("storage failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)

		ctx := authn.SetInfo(t.Context(), Session{UserId: "u1"})
		store.EXPECT().DeleteUser(gomock.Any(), persistence.DeleteUserArg{UserID: "u1"}).Return(persistence.DeleteUserRet{}, errors.New("boom"))

		_, err := h.DeleteUser(ctx, &pb.DeleteUserRequest{})
		require.Error(t, err)
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)

		ctx := authn.SetInfo(t.Context(), Session{UserId: "u1"})
		store.EXPECT().DeleteUser(gomock.Any(), persistence.DeleteUserArg{UserID: "u1"}).Return(persistence.DeleteUserRet{User: persistence.User{ID: "u1"}}, nil)

		resp, err := h.DeleteUser(ctx, &pb.DeleteUserRequest{})
		require.NoError(t, err)
		require.Equal(t, "u1", resp.User.Id)
	})
}

func TestHandlerGetSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := persistencemock.NewMockTransactionalStorage(ctrl)
	h := newTestHandler(t, store)

	store.EXPECT().GetSession(gomock.Any(), gomock.Any()).Return(persistence.GetSessionRet{}, errors.New("boom"))

	_, err := h.getSession(t.Context(), "token")
	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestHandlerCreateInvitation(t *testing.T) {
	t.Run("missing session", func(t *testing.T) {
		h := &Handler{secret: "secret"}

		_, err := h.CreateInvitation(t.Context(), &pb.CreateInvitationRequest{})
		require.Error(t, err)
		require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("stores hashed token", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)
		ctx := authn.SetInfo(t.Context(), Session{UserId: "inviter-1"})

		var insertArg persistence.InsertVerificationArg
		store.EXPECT().InsertVerification(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, in persistence.InsertVerificationArg) (persistence.InsertVerificationRet, error) {
				insertArg = in
				require.NotEmpty(t, in.TokenHash)
				require.Equal(t, "inviter-1", in.AuthorID)
				require.Equal(t, defaultInvitationMaxUses, in.MaxUses)
				require.WithinDuration(t, time.Now().Add(defaultInvitationDuration), in.ExpiresAt, 2*time.Second)
				return persistence.InsertVerificationRet{Verification: persistence.Verification{ExpiresAt: in.ExpiresAt, MaxUses: in.MaxUses}}, nil
			},
		)

		resp, err := h.CreateInvitation(ctx, &pb.CreateInvitationRequest{})
		require.NoError(t, err)
		require.NotEmpty(t, resp.InvitationToken)
		require.Equal(t, int32(defaultInvitationMaxUses), resp.MaxUses)
		require.NotNil(t, resp.ExpiresAt)

		gotTokenHash, err := tokenHash(resp.InvitationToken, "secret")
		require.NoError(t, err)
		require.Equal(t, insertArg.TokenHash, gotTokenHash)
		require.NotEqual(t, resp.InvitationToken, insertArg.TokenHash)
	})

	t.Run("storage failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := persistencemock.NewMockTransactionalStorage(ctrl)
		h := newTestHandler(t, store)
		ctx := authn.SetInfo(t.Context(), Session{UserId: "inviter-1"})

		store.EXPECT().InsertVerification(gomock.Any(), gomock.Any()).Return(persistence.InsertVerificationRet{}, errors.New("boom"))

		_, err := h.CreateInvitation(ctx, &pb.CreateInvitationRequest{})
		require.Error(t, err)
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	})
}
