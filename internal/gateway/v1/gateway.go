package v1

import (
	"context"
	"errors"
	"log/slog"
	"net/url"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	v1 "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/api/gateway/v1/gatewayv1connect"
	"github.com/stiflerGit/moviehat/internal/auth"
	"github.com/stiflerGit/moviehat/internal/list"
	"github.com/stiflerGit/moviehat/internal/session"
	"github.com/stiflerGit/moviehat/internal/user"
)

//go:generate mockgen -package mocks -destination mocks/mocks.go -source=./gateway.go

// AuthHandler defines the authentication operations used by the gateway.
type AuthHandler interface {
	CreateInvitation(context.Context, *v1.CreateInvitationRequest) (*v1.CreateInvitationResponse, error)
	SignUp(ctx context.Context, req *v1.SignUpRequest) (auth.SignUpResponse, error)
	SignIn(ctx context.Context, req *v1.SignInRequest) (*v1.SignInResponse, error)
	SignOut(ctx context.Context, req *v1.SignOutRequest) (*v1.SignOutResponse, error)
	DeleteUser(ctx context.Context, req *v1.DeleteUserRequest) (*v1.DeleteUserResponse, error)
}

// UsersHandler defines the MovieHat operations used by the gateway.
type UsersHandler interface {
	CreateUser(context.Context, user.CreateUserRequest) (user.CreateUserResponse, error)
	ListUsers(context.Context, *v1.ListUsersRequest) (*v1.ListUsersResponse, error)
	UpdateUser(context.Context, user.UpdateUserRequest) (*v1.UpdateUserResponse, error)
	DeleteUser(context.Context, user.DeleteUserRequest) (*v1.DeleteUserResponse, error)
}

// TODO: SessionsHandler
type SessionsHandler interface {
	CreateSession(context.Context, *v1.CreateSessionRequest) (*v1.CreateSessionResponse, error)
	ListSessions(context.Context, *v1.ListSessionsRequest) (*v1.ListSessionsResponse, error)
	GetSession(context.Context, *v1.GetSessionRequest) (*v1.GetSessionResponse, error)
	EndSession(context.Context, *v1.EndSessionRequest) (*v1.EndSessionResponse, error)
	SetSessionMovie(context.Context, session.SetSessionMovieRequest) (*v1.SetSessionMovieResponse, error)
	GetSessionProbabilities(context.Context, *v1.GetSessionProbabilitiesRequest) (*v1.GetSessionProbabilitiesResponse, error)
	DeleteSession(context.Context, *v1.DeleteSessionRequest) (*v1.DeleteSessionResponse, error)
	AddParticipant(context.Context, *v1.AddParticipantRequest) (*v1.AddParticipantResponse, error)
	RemoveParticipant(context.Context, *v1.RemoveParticipantRequest) (*v1.RemoveParticipantResponse, error)
	ListParticipants(context.Context, *v1.ListParticipantsRequest) (*v1.ListParticipantsResponse, error)
}

// TODO: ListsHandler
type ListsHandler interface {
	AddUserMovie(context.Context, list.AddUserMovieRequest) (*v1.AddUserMovieResponse, error)
	ListUserMovies(context.Context, *v1.ListUserMoviesRequest) (*v1.ListUserMoviesResponse, error)
	DeleteUserMovie(context.Context, list.DeleteUserMovieRequest) (*v1.DeleteUserMovieResponse, error)
}

// TODO: MovieHandler
type MovieHandler interface {
	SearchMovie(context.Context, *v1.SearchMovieRequest) (*v1.SearchMovieResponse, error)
}

// Handler implements the GatewayService RPC surface.
type Handler struct {
	AuthHandler
	UsersHandler
	SessionsHandler
	ListsHandler
	MovieHandler
	frontendInvitationURL *url.URL
	logger                *slog.Logger
}

var _ gatewayv1connect.GatewayServiceHandler = (*Handler)(nil)

// New creates a gateway handler.
func New(
	authHandler AuthHandler,
	usersHandler UsersHandler,
	sessionsHandler SessionsHandler,
	movieListsHandler ListsHandler,
	movieSearchHandler MovieHandler,
	options ...Option,
) *Handler {
	h := &Handler{
		AuthHandler:           authHandler,
		frontendInvitationURL: &url.URL{},
		UsersHandler:          usersHandler,
		SessionsHandler:       sessionsHandler,
		ListsHandler:          movieListsHandler,
		MovieHandler:          movieSearchHandler,
		logger:                slog.Default().With("component", "gateway"),
	}

	for _, opt := range options {
		opt(h)
	}

	return h
}

// CreateInvitation creates an invitation link for a new user.
func (h *Handler) CreateInvitation(ctx context.Context, req *v1.CreateInvitationRequest) (*v1.CreateInvitationResponse, error) {
	resp, err := h.AuthHandler.CreateInvitation(ctx, req)
	if err != nil {
		h.logger.ErrorContext(ctx, "h.authHandler.CreateInvitation", "error", err)
		return nil, repoErrorToAPIError(err)
	}

	resp.InvitationUrl = buildInvitationURL(h.frontendInvitationURL, resp.InvitationToken)
	return resp, nil
}

// SignUp creates an invited user account.
func (h *Handler) SignUp(ctx context.Context, req *v1.SignUpRequest) (*v1.SignUpResponse, error) {
	signupResp, err := h.AuthHandler.SignUp(ctx, req)
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	_, err = h.CreateUser(ctx, user.CreateUserRequest{UserID: signupResp.UserID})
	if err != nil {
		h.logger.ErrorContext(ctx, "sign up created auth user but failed to create moviehat user", "user_id", signupResp.UserID, "error", err)
		return nil, repoErrorToAPIError(err)
	}
	return &v1.SignUpResponse{Token: signupResp.Token}, nil
}

// UpdateUser update the info of the user
//
// TODO: for now they can only update name. We should allow to update everything (email, password, etc)
// maybe we can separate things (UpdateProfile -> addional info, UpdateUser -> session related info)
func (h *Handler) UpdateUser(ctx context.Context, req *v1.UpdateUserRequest) (*v1.UpdateUserResponse, error) {
	session, ok := authn.GetInfo(ctx).(auth.Session)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing session"))
	}

	updateUserResp, err := h.UsersHandler.UpdateUser(ctx, user.UpdateUserRequest{UserID: session.UserId, Name: req.Name})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	return updateUserResp, nil
}

// DeleteUser deletes the current user from MovieHat and auth storage.
func (h *Handler) DeleteUser(ctx context.Context, req *v1.DeleteUserRequest) (*v1.DeleteUserResponse, error) {
	session, ok := authn.GetInfo(ctx).(auth.Session)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing session"))
	}

	movieHatDeleteUserResp, err := h.UsersHandler.DeleteUser(ctx, user.DeleteUserRequest{UserID: session.UserId})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	_, err = h.AuthHandler.DeleteUser(ctx, req)
	if err != nil {
		h.logger.ErrorContext(ctx, "delete user partially failed: moviehat user deleted but auth user delete failed", "error", err)
		return movieHatDeleteUserResp, nil
	}

	return movieHatDeleteUserResp, nil
}

func (h *Handler) AddUserMovie(ctx context.Context, req *v1.AddUserMovieRequest) (*v1.AddUserMovieResponse, error) {
	session, ok := authn.GetInfo(ctx).(auth.Session)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing session"))
	}

	addUserMovieResp, err := h.ListsHandler.AddUserMovie(ctx, list.AddUserMovieRequest{UserID: session.UserId, MovieID: req.MovieId, Note: req.Note})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	return addUserMovieResp, nil
}

func (h *Handler) DeleteUserMovie(ctx context.Context, req *v1.DeleteUserMovieRequest) (*v1.DeleteUserMovieResponse, error) {
	session, ok := authn.GetInfo(ctx).(auth.Session)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing session"))
	}

	deleteUserMovieResponse, err := h.ListsHandler.DeleteUserMovie(ctx, list.DeleteUserMovieRequest{UserID: session.UserId, MovieID: req.Id})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	return deleteUserMovieResponse, nil
}

func (h *Handler) SetSessionMovie(ctx context.Context, req *v1.SetSessionMovieRequest) (*v1.SetSessionMovieResponse, error) {
	authSession, ok := authn.GetInfo(ctx).(auth.Session)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing auth session"))
	}

	setSessionMovieResponse, err := h.SessionsHandler.SetSessionMovie(ctx, session.SetSessionMovieRequest{UserID: authSession.UserId, SetSessionMovieRequest: req})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	return setSessionMovieResponse, nil
}

func buildInvitationURL(baseURL *url.URL, token string) string {
	ret := baseURL.Clone()
	ret.Fragment = token
	return ret.String()
}
