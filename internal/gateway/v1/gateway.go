package v1

import (
	"context"
	"log/slog"
	"net/url"

	v1 "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/api/gateway/v1/gatewayv1connect"
	"github.com/stiflerGit/moviehat/internal/auth"
	moviehat "github.com/stiflerGit/moviehat/internal/moviehat"
)

//go:generate mockgen -package mocks -destination mocks/auth_handler.go . AuthHandler
//go:generate mockgen -package mocks -destination mocks/moviehat_handler.go . MovieHatHandler

// AuthHandler defines the authentication operations used by the gateway.
type AuthHandler interface {
	CreateInvitation(context.Context, *v1.CreateInvitationRequest) (*v1.CreateInvitationResponse, error)
	SignUp(ctx context.Context, req *v1.SignUpRequest) (auth.SignUpResponse, error)
	SignIn(ctx context.Context, req *v1.SignInRequest) (*v1.SignInResponse, error)
	SignOut(ctx context.Context, req *v1.SignOutRequest) (*v1.SignOutResponse, error)
	DeleteUser(ctx context.Context, req *v1.DeleteUserRequest) (*v1.DeleteUserResponse, error)
}

// MovieHatHandler defines the MovieHat operations used by the gateway.
type MovieHatHandler interface {
	CreateUser(ctx context.Context, req moviehat.CreateUserRequest) (moviehat.CreateUserResponse, error)
	ListUsers(ctx context.Context, req *v1.ListUsersRequest) (*v1.ListUsersResponse, error)
	UpdateUser(ctx context.Context, req *v1.UpdateUserRequest) (*v1.UpdateUserResponse, error)
	DeleteUser(ctx context.Context, req *v1.DeleteUserRequest) (*v1.DeleteUserResponse, error)
	CreateSession(ctx context.Context, req *v1.CreateSessionRequest) (*v1.CreateSessionResponse, error)
	ListSessions(ctx context.Context, req *v1.ListSessionsRequest) (*v1.ListSessionsResponse, error)
	GetSession(ctx context.Context, req *v1.GetSessionRequest) (*v1.GetSessionResponse, error)
	EndSession(ctx context.Context, req *v1.EndSessionRequest) (*v1.EndSessionResponse, error)
	SetSessionMovie(context.Context, *v1.SetSessionMovieRequest) (*v1.SetSessionMovieResponse, error)
	DeleteSession(ctx context.Context, req *v1.DeleteSessionRequest) (*v1.DeleteSessionResponse, error)
	AddParticipant(ctx context.Context, req *v1.AddParticipantRequest) (*v1.AddParticipantResponse, error)
	RemoveParticipant(ctx context.Context, req *v1.RemoveParticipantRequest) (*v1.RemoveParticipantResponse, error)
	ListParticipants(ctx context.Context, req *v1.ListParticipantsRequest) (*v1.ListParticipantsResponse, error)
	AddUserMovie(ctx context.Context, req *v1.AddUserMovieRequest) (*v1.AddUserMovieResponse, error)
	ListUserMovies(ctx context.Context, req *v1.ListUserMoviesRequest) (*v1.ListUserMoviesResponse, error)
	DeleteUserMovie(ctx context.Context, req *v1.DeleteUserMovieRequest) (*v1.DeleteUserMovieResponse, error)
}

// Handler implements the GatewayService RPC surface.
type Handler struct {
	authHandler           AuthHandler
	frontendInvitationURL *url.URL
	movieHatHandler       MovieHatHandler
	logger                *slog.Logger
}

var _ gatewayv1connect.GatewayServiceHandler = (*Handler)(nil)

// New creates a gateway handler.
func New(authHandler AuthHandler, movieHatHandler MovieHatHandler, options ...Option) *Handler {
	h := &Handler{
		authHandler:           authHandler,
		movieHatHandler:       movieHatHandler,
		frontendInvitationURL: &url.URL{},
		logger:                slog.Default().With("component", "gateway"),
	}

	for _, opt := range options {
		opt(h)
	}

	return h
}

// CreateInvitation creates an invitation link for a new user.
func (h *Handler) CreateInvitation(ctx context.Context, req *v1.CreateInvitationRequest) (*v1.CreateInvitationResponse, error) {
	resp, err := h.authHandler.CreateInvitation(ctx, req)
	if err != nil {
		h.logger.ErrorContext(ctx, "h.authHandler.CreateInvitation", "error", err)
		return nil, err
	}

	resp.InvitationUrl = buildInvitationURL(h.frontendInvitationURL, resp.InvitationToken)
	return resp, nil
}

// SignUp creates an invited user account.
func (h *Handler) SignUp(ctx context.Context, req *v1.SignUpRequest) (*v1.SignUpResponse, error) {
	signupResp, err := h.authHandler.SignUp(ctx, req)
	if err != nil {
		return nil, err
	}

	_, err = h.movieHatHandler.CreateUser(ctx, moviehat.CreateUserRequest{UserID: signupResp.UserID})
	if err != nil {
		h.logger.ErrorContext(ctx, "sign up created auth user but failed to create moviehat user", "user_id", signupResp.UserID, "error", err)
		return nil, err
	}
	return &v1.SignUpResponse{Token: signupResp.Token}, nil
}

// SignIn authenticates a user.
func (h *Handler) SignIn(ctx context.Context, req *v1.SignInRequest) (*v1.SignInResponse, error) {
	return h.authHandler.SignIn(ctx, req)
}

// SignOut expires the current session.
func (h *Handler) SignOut(ctx context.Context, req *v1.SignOutRequest) (*v1.SignOutResponse, error) {
	return h.authHandler.SignOut(ctx, req)
}

// ListUsers lists MovieHat users.
func (h *Handler) ListUsers(ctx context.Context, req *v1.ListUsersRequest) (*v1.ListUsersResponse, error) {
	return h.movieHatHandler.ListUsers(ctx, req)
}

// UpdateUser updates the current MovieHat user.
func (h *Handler) UpdateUser(ctx context.Context, req *v1.UpdateUserRequest) (*v1.UpdateUserResponse, error) {
	return h.movieHatHandler.UpdateUser(ctx, req)
}

// DeleteUser deletes the current user from MovieHat and auth storage.
func (h *Handler) DeleteUser(ctx context.Context, req *v1.DeleteUserRequest) (*v1.DeleteUserResponse, error) {
	movieHatDeleteUserResp, err := h.movieHatHandler.DeleteUser(ctx, req)
	if err != nil {
		return nil, err
	}

	_, err = h.authHandler.DeleteUser(ctx, req)
	if err != nil {
		h.logger.ErrorContext(ctx, "delete user partially failed: moviehat user deleted but auth user delete failed", "error", err)
		return movieHatDeleteUserResp, nil
	}

	return movieHatDeleteUserResp, nil
}

// CreateSession creates a movie selection session.
func (h *Handler) CreateSession(ctx context.Context, req *v1.CreateSessionRequest) (*v1.CreateSessionResponse, error) {
	return h.movieHatHandler.CreateSession(ctx, req)
}

// ListSessions lists movie selection sessions.
func (h *Handler) ListSessions(ctx context.Context, req *v1.ListSessionsRequest) (*v1.ListSessionsResponse, error) {
	return h.movieHatHandler.ListSessions(ctx, req)
}

// GetSession returns a movie selection session.
func (h *Handler) GetSession(ctx context.Context, req *v1.GetSessionRequest) (*v1.GetSessionResponse, error) {
	return h.movieHatHandler.GetSession(ctx, req)
}

// EndSession closes a movie selection session and returns its winner.
func (h *Handler) EndSession(ctx context.Context, req *v1.EndSessionRequest) (*v1.EndSessionResponse, error) {
	return h.movieHatHandler.EndSession(ctx, req)
}

// SetSessionMovie records the watched movie for a closed session.
func (h *Handler) SetSessionMovie(ctx context.Context, req *v1.SetSessionMovieRequest) (*v1.SetSessionMovieResponse, error) {
	return h.movieHatHandler.SetSessionMovie(ctx, req)
}

// DeleteSession deletes a movie selection session.
func (h *Handler) DeleteSession(ctx context.Context, req *v1.DeleteSessionRequest) (*v1.DeleteSessionResponse, error) {
	return h.movieHatHandler.DeleteSession(ctx, req)
}

// AddParticipant adds a user to a movie selection session.
func (h *Handler) AddParticipant(ctx context.Context, req *v1.AddParticipantRequest) (*v1.AddParticipantResponse, error) {
	return h.movieHatHandler.AddParticipant(ctx, req)
}

// RemoveParticipant removes a user from a movie selection session.
func (h *Handler) RemoveParticipant(ctx context.Context, req *v1.RemoveParticipantRequest) (*v1.RemoveParticipantResponse, error) {
	return h.movieHatHandler.RemoveParticipant(ctx, req)
}

// ListParticipants lists the users in a movie selection session.
func (h *Handler) ListParticipants(ctx context.Context, req *v1.ListParticipantsRequest) (*v1.ListParticipantsResponse, error) {
	return h.movieHatHandler.ListParticipants(ctx, req)
}

// AddUserMovie adds a movie to the current user's list.
func (h *Handler) AddUserMovie(ctx context.Context, req *v1.AddUserMovieRequest) (*v1.AddUserMovieResponse, error) {
	return h.movieHatHandler.AddUserMovie(ctx, req)
}

// ListUserMovies lists movies for a user.
func (h *Handler) ListUserMovies(ctx context.Context, req *v1.ListUserMoviesRequest) (*v1.ListUserMoviesResponse, error) {
	return h.movieHatHandler.ListUserMovies(ctx, req)
}

// DeleteUserMovie deletes a movie from the current user's list.
func (h *Handler) DeleteUserMovie(ctx context.Context, req *v1.DeleteUserMovieRequest) (*v1.DeleteUserMovieResponse, error) {
	return h.movieHatHandler.DeleteUserMovie(ctx, req)
}

func buildInvitationURL(baseURL *url.URL, token string) string {
	// TODO: use baseURL.Clone in go 1.27
	ret, _ := url.Parse(baseURL.String())
	ret.Fragment = token
	return ret.String()
}
