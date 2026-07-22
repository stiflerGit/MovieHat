package v1

import (
	"context"
	"errors"
	"net/url"
	"testing"

	gatewaypb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/auth"
	gatewaymock "github.com/stiflerGit/moviehat/internal/gateway/v1/mocks"
	moviehat "github.com/stiflerGit/moviehat/internal/moviehat"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNew(t *testing.T) {
	h := New(nil, nil)
	require.NotNil(t, h)
}

func TestHandlerSignUp(t *testing.T) {
	t.Run("auth error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		authHandler := gatewaymock.NewMockAuthHandler(ctrl)
		movieHatHandler := gatewaymock.NewMockMovieHatHandler(ctrl)
		h := New(authHandler, movieHatHandler)

		authHandler.EXPECT().SignUp(gomock.Any(), gomock.Any()).Return(auth.SignUpResponse{}, errors.New("boom"))

		_, err := h.SignUp(context.Background(), &gatewaypb.SignUpRequest{Email: "a@b.com", Password: "password1"})
		require.EqualError(t, err, "boom")
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		authHandler := gatewaymock.NewMockAuthHandler(ctrl)
		movieHatHandler := gatewaymock.NewMockMovieHatHandler(ctrl)
		h := New(authHandler, movieHatHandler)

		authHandler.EXPECT().SignUp(gomock.Any(), gomock.Any()).Return(auth.SignUpResponse{UserID: "u1"}, nil)
		movieHatHandler.EXPECT().CreateUser(gomock.Any(), moviehat.CreateUserRequest{UserID: "u1"}).Return(moviehat.CreateUserResponse{}, nil)

		resp, err := h.SignUp(context.Background(), &gatewaypb.SignUpRequest{Email: "a@b.com", Password: "password1"})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestHandlerSignIn(t *testing.T) {
	ctrl := gomock.NewController(t)
	authHandler := gatewaymock.NewMockAuthHandler(ctrl)
	movieHatHandler := gatewaymock.NewMockMovieHatHandler(ctrl)
	h := New(authHandler, movieHatHandler)

	authHandler.EXPECT().SignIn(gomock.Any(), &gatewaypb.SignInRequest{Email: "a@b.com", Password: "password1"}).
		Return(&gatewaypb.SignInResponse{Token: "token"}, nil)

	resp, err := h.SignIn(context.Background(), &gatewaypb.SignInRequest{Email: "a@b.com", Password: "password1"})
	require.NoError(t, err)
	require.Equal(t, "token", resp.Token)
}

func TestHandlerListUsers(t *testing.T) {
	ctrl := gomock.NewController(t)
	authHandler := gatewaymock.NewMockAuthHandler(ctrl)
	movieHatHandler := gatewaymock.NewMockMovieHatHandler(ctrl)
	h := New(authHandler, movieHatHandler)

	movieHatHandler.EXPECT().ListUsers(gomock.Any(), &gatewaypb.ListUsersRequest{}).
		Return(&gatewaypb.ListUsersResponse{Users: []*gatewaypb.User{{Id: "u1"}}}, nil)

	resp, err := h.ListUsers(context.Background(), &gatewaypb.ListUsersRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Users, 1)
	require.Equal(t, "u1", resp.Users[0].Id)
}

func TestHandlerDeleteUser(t *testing.T) {
	t.Run("moviehat delete failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		authHandler := gatewaymock.NewMockAuthHandler(ctrl)
		movieHatHandler := gatewaymock.NewMockMovieHatHandler(ctrl)
		h := New(authHandler, movieHatHandler)

		movieHatHandler.EXPECT().DeleteUser(gomock.Any(), &gatewaypb.DeleteUserRequest{}).Return(nil, errors.New("boom"))

		_, err := h.DeleteUser(context.Background(), &gatewaypb.DeleteUserRequest{})
		require.EqualError(t, err, "boom")
	})

	t.Run("auth delete failure is best effort", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		authHandler := gatewaymock.NewMockAuthHandler(ctrl)
		movieHatHandler := gatewaymock.NewMockMovieHatHandler(ctrl)
		h := New(authHandler, movieHatHandler)

		movieHatHandler.EXPECT().DeleteUser(gomock.Any(), &gatewaypb.DeleteUserRequest{}).Return(&gatewaypb.DeleteUserResponse{User: &gatewaypb.User{Id: "u1"}}, nil)
		authHandler.EXPECT().DeleteUser(gomock.Any(), &gatewaypb.DeleteUserRequest{}).Return(nil, errors.New("boom-auth"))

		resp, err := h.DeleteUser(context.Background(), &gatewaypb.DeleteUserRequest{})
		require.NoError(t, err)
		require.Equal(t, "u1", resp.User.Id)
	})
}

func TestHandlerCreateInvitation(t *testing.T) {
	t.Run("auth error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		authHandler := gatewaymock.NewMockAuthHandler(ctrl)
		movieHatHandler := gatewaymock.NewMockMovieHatHandler(ctrl)
		h := New(authHandler, movieHatHandler)

		authHandler.EXPECT().CreateInvitation(gomock.Any(), &gatewaypb.CreateInvitationRequest{}).Return(nil, errors.New("boom"))

		_, err := h.CreateInvitation(t.Context(), &gatewaypb.CreateInvitationRequest{})
		require.EqualError(t, err, "boom")
	})

	t.Run("adds frontend invitation url", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		authHandler := gatewaymock.NewMockAuthHandler(ctrl)
		movieHatHandler := gatewaymock.NewMockMovieHatHandler(ctrl)
		baseURL, err := url.Parse("https://app.example.com/invite")
		require.NoError(t, err)
		h := New(authHandler, movieHatHandler, WithInvitationBaseURL(baseURL))

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
