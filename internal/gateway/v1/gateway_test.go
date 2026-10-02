package v1

import (
	"context"
	"errors"
	"net/url"
	"testing"

	gatewaypb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/auth"
	"github.com/stiflerGit/moviehat/internal/gateway/v1/mocks"
	"github.com/stiflerGit/moviehat/internal/user"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNew(t *testing.T) {
	h := New(nil, nil, nil, nil, nil)
	require.NotNil(t, h)
}

func TestHandlerSignUp(t *testing.T) {
	t.Run("auth error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		authHandler := mocks.NewMockAuthHandler(ctrl)
		userHandler := mocks.NewMockUsersHandler(ctrl)
		h := New(authHandler, userHandler, nil, nil, nil)

		authHandler.EXPECT().SignUp(gomock.Any(), gomock.Any()).Return(auth.SignUpResponse{}, errors.New("boom"))

		_, err := h.SignUp(context.Background(), &gatewaypb.SignUpRequest{Email: "a@b.com", Password: "password1"})
		require.Error(t, err)
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		require.ErrorContains(t, err, "boom")
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		authHandler := mocks.NewMockAuthHandler(ctrl)
		userHandler := mocks.NewMockUsersHandler(ctrl)
		h := New(authHandler, userHandler, nil, nil, nil)

		authHandler.EXPECT().SignUp(gomock.Any(), gomock.Any()).Return(auth.SignUpResponse{UserID: "u1"}, nil)
		userHandler.EXPECT().CreateUser(gomock.Any(), user.CreateUserRequest{UserID: "u1"}).Return(user.CreateUserResponse{}, nil)

		resp, err := h.SignUp(context.Background(), &gatewaypb.SignUpRequest{Email: "a@b.com", Password: "password1"})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestHandlerSignIn(t *testing.T) {
	ctrl := gomock.NewController(t)
	authHandler := mocks.NewMockAuthHandler(ctrl)
	userHandler := mocks.NewMockUsersHandler(ctrl)
	h := New(authHandler, userHandler, nil, nil, nil)

	authHandler.EXPECT().SignIn(gomock.Any(), &gatewaypb.SignInRequest{Email: "a@b.com", Password: "password1"}).
		Return(&gatewaypb.SignInResponse{Token: "token"}, nil)

	resp, err := h.SignIn(context.Background(), &gatewaypb.SignInRequest{Email: "a@b.com", Password: "password1"})
	require.NoError(t, err)
	require.Equal(t, "token", resp.Token)
}

func TestHandlerListUsers(t *testing.T) {
	ctrl := gomock.NewController(t)
	authHandler := mocks.NewMockAuthHandler(ctrl)
	userHandler := mocks.NewMockUsersHandler(ctrl)
	h := New(authHandler, userHandler, nil, nil, nil)

	userHandler.EXPECT().ListUsers(gomock.Any(), &gatewaypb.ListUsersRequest{}).
		Return(&gatewaypb.ListUsersResponse{Users: []*gatewaypb.User{{Id: "u1"}}}, nil)

	resp, err := h.ListUsers(context.Background(), &gatewaypb.ListUsersRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Users, 1)
	require.Equal(t, "u1", resp.Users[0].Id)
}

func TestHandlerDeleteUser(t *testing.T) {
	userCtx := func() context.Context {
		return authn.SetInfo(context.Background(), auth.Session{UserId: "u1"})
	}

	t.Run("missing session", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		authHandler := mocks.NewMockAuthHandler(ctrl)
		userHandler := mocks.NewMockUsersHandler(ctrl)
		h := New(authHandler, userHandler, nil, nil, nil)

		_, err := h.DeleteUser(context.Background(), &gatewaypb.DeleteUserRequest{})
		require.Error(t, err)
		require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("moviehat delete failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		authHandler := mocks.NewMockAuthHandler(ctrl)
		userHandler := mocks.NewMockUsersHandler(ctrl)
		h := New(authHandler, userHandler, nil, nil, nil)

		userHandler.EXPECT().DeleteUser(gomock.Any(), user.DeleteUserRequest{UserID: "u1"}).Return(nil, errors.New("boom"))

		_, err := h.DeleteUser(userCtx(), &gatewaypb.DeleteUserRequest{})
		require.Error(t, err)
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		require.ErrorContains(t, err, "boom")
	})

	t.Run("auth delete failure is best effort", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		authHandler := mocks.NewMockAuthHandler(ctrl)
		userHandler := mocks.NewMockUsersHandler(ctrl)
		h := New(authHandler, userHandler, nil, nil, nil)

		userHandler.EXPECT().DeleteUser(gomock.Any(), user.DeleteUserRequest{UserID: "u1"}).Return(&gatewaypb.DeleteUserResponse{User: &gatewaypb.User{Id: "u1"}}, nil)
		authHandler.EXPECT().DeleteUser(gomock.Any(), &gatewaypb.DeleteUserRequest{}).Return(nil, errors.New("boom-auth"))

		resp, err := h.DeleteUser(userCtx(), &gatewaypb.DeleteUserRequest{})
		require.NoError(t, err)
		require.Equal(t, "u1", resp.User.Id)
	})
}

func TestHandlerCreateInvitation(t *testing.T) {
	t.Run("auth error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		authHandler := mocks.NewMockAuthHandler(ctrl)
		h := New(authHandler, nil, nil, nil, nil)

		authHandler.EXPECT().CreateInvitation(gomock.Any(), &gatewaypb.CreateInvitationRequest{}).Return(nil, errors.New("boom"))

		_, err := h.CreateInvitation(t.Context(), &gatewaypb.CreateInvitationRequest{})
		require.Error(t, err)
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		require.ErrorContains(t, err, "boom")
	})

	t.Run("adds frontend invitation url", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		authHandler := mocks.NewMockAuthHandler(ctrl)
		baseURL, err := url.Parse("https://app.example.com/invite")
		require.NoError(t, err)
		h := New(authHandler, nil, nil, nil, nil, WithInvitationBaseURL(baseURL))

		authHandler.EXPECT().CreateInvitation(gomock.Any(), &gatewaypb.CreateInvitationRequest{}).Return(&gatewaypb.CreateInvitationResponse{InvitationToken: "invite-token"}, nil)

		resp, err := h.CreateInvitation(t.Context(), &gatewaypb.CreateInvitationRequest{})
		require.NoError(t, err)
		require.Equal(t, "invite-token", resp.InvitationToken)
		require.Equal(t, "https://app.example.com/invite#invite-token", resp.InvitationUrl)
	})
}

func TestBuildInvitationURL(t *testing.T) {
	baseURL, err := url.Parse("https://app.example.com/invite")
	require.NoError(t, err)

	got := buildInvitationURL(baseURL, "invite-token")
	require.Equal(t, "https://app.example.com/invite#invite-token", got)
	require.Empty(t, baseURL.Fragment)
}
