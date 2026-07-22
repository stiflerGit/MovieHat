package v1

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/auth"
	"github.com/stiflerGit/moviehat/internal/moviehat/persistence"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
)

// Extractor selects and records winners for closed sessions.
//
//go:generate mockgen -package mocks -destination mocks/extractor.go . Extractor
type Extractor interface {
	Extract(ctx context.Context, session *pb.Session) (*pb.User, error)
	StoreExtraction(ctx context.Context, session *pb.Session) error
}

// Handler provides MovieHat user, movie, and session operations.
type Handler struct {
	repository persistence.TransactionalStorage
	extractor  Extractor
	logger     *slog.Logger
}

// New creates a MovieHat handler.
func New(repository persistence.TransactionalStorage, extractionHandler Extractor, options ...Option) *Handler {
	h := &Handler{
		repository: repository,
		extractor:  extractionHandler,
		logger:     slog.Default().With("component", "server"),
	}

	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(h)
	}

	return h
}

// CreateUserRequest contains the auth user id to initialize in MovieHat.
type CreateUserRequest struct {
	UserID string
}

// CreateUserResponse contains the created MovieHat user.
type CreateUserResponse struct {
	User *pb.User
}

// CreateUser creates a MovieHat user for an auth user.
func (h *Handler) CreateUser(ctx context.Context, req CreateUserRequest) (CreateUserResponse, error) {
	user, err := h.repository.CreateUser(ctx, persistence.CreateUserArg{UserID: req.UserID})
	if err != nil {
		return CreateUserResponse{}, repoErrorToAPIError(err)
	}

	h.logger.InfoContext(ctx, "user created", "user_id", user.ID)

	return CreateUserResponse{User: repoUserToPBUser(user)}, nil
}

// ListUsers lists MovieHat users.
func (h *Handler) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	users, err := h.repository.ListUsers(ctx, persistence.ListUsersArg{})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	v1Users := make([]*pb.User, 0, len(users))
	for _, u := range users {
		v1Users = append(v1Users, &pb.User{
			Id:   u.ID,
			Name: u.Name,
		})
	}

	return &pb.ListUsersResponse{Users: v1Users}, nil
}

// UpdateUser updates the current user's MovieHat profile.
func (h *Handler) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	if req.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is empty"))
	}

	// Should this be in gateway instead? and here we pass explicit userID
	session, ok := authn.GetInfo(ctx).(auth.Session)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing session"))
	}

	user, err := h.repository.UpdateUser(ctx, persistence.UpdateUserArg{UserID: session.UserId, Name: req.Name})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	h.logger.InfoContext(ctx, "user updated", "user_id", user.ID)

	return &pb.UpdateUserResponse{
		User: &pb.User{
			Id:   user.ID,
			Name: user.Name,
		},
	}, nil
}

// DeleteUser deletes the current user's MovieHat profile.
func (h *Handler) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	// Should this be in gateway instead? and here we pass explicit userID
	session, ok := authn.GetInfo(ctx).(auth.Session)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing session"))
	}

	user, err := h.repository.DeleteUser(ctx, persistence.DeleteUserArg{UserID: session.UserId})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	h.logger.InfoContext(ctx, "user deleted", "user_id", user.ID)

	return &pb.DeleteUserResponse{
		User: &pb.User{
			Id:   user.ID,
			Name: user.Name,
		},
	}, nil
}

// CreateSession creates a movie selection session.
func (h *Handler) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
	session, err := h.repository.CreateSession(ctx, persistence.CreateSessionArg{})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	h.logger.InfoContext(ctx, "session created", "session_id", session.ID)
	return &pb.CreateSessionResponse{Session: repoSessionToPBSession(session)}, nil
}

// ListSessions lists movie selection sessions.
func (h *Handler) ListSessions(ctx context.Context, req *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	repoSessions, err := h.repository.ListSessions(ctx, persistence.ListSessionsAg{})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	return &pb.ListSessionsResponse{Sessions: repoSessionsToPB(repoSessions.Sessions...)}, nil
}

// GetSession returns a movie selection session.
func (h *Handler) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.GetSessionResponse, error) {
	if req.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is empty"))
	}

	session, err := h.repository.GetSession(ctx, persistence.GetSessionArg{ID: req.Id})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	participants, err := h.repository.ListParticipants(ctx, persistence.ListParticipantsArg{SessionID: req.Id})
	if err != nil && !errors.Is(err, persistence.ErrNotFound) {
		return nil, repoErrorToAPIError(err)
	}

	pbSession := repoSessionToPBSession(session)
	pbSession.Participants = repoUsersToPBUsers(participants.Participants...)
	return &pb.GetSessionResponse{Session: pbSession}, nil
}

// EndSession ends a session, extracting a winner among participants of the session and store results
// EndSession closes a movie selection session and returns its winner.
func (h *Handler) EndSession(ctx context.Context, req *pb.EndSessionRequest) (*pb.EndSessionResponse, error) {
	if req.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is empty"))
	}

	var winnerUser *pb.User

	var repoSession persistence.Session
	var repoParticipants []persistence.User
	err := h.repository.WithTx(ctx, func(ctx context.Context, s persistence.Storage) error {
		err := s.CloseSession(ctx, req.Id)
		if err != nil {
			return fmt.Errorf("r.CloseSession: %w", err)
		}

		listParticipantsRet, err := s.ListParticipants(ctx, persistence.ListParticipantsArg{SessionID: req.Id})
		if err != nil {
			return fmt.Errorf("r.ListParticipants: %w", err)
		}

		if len(listParticipantsRet.Participants) < 2 {
			// no participant for the session. Cannot be closed, just deleted
			return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("not enough participants: count=%d", len(listParticipantsRet.Participants)))
		}

		repoParticipants = listParticipantsRet.Participants
		pbSession := &pb.Session{Participants: repoUsersToPBUsers(repoParticipants...)}

		winnerUser, err = h.extractor.Extract(ctx, pbSession)
		if err != nil {
			return err
		}

		if winnerUser == nil || winnerUser.Id == "" {
			return errors.New("extractor returned empty winner")
		}

		repoSession, err = s.UpdateSession(ctx, persistence.UpdateSessionArg{ID: req.Id, WinnerID: &winnerUser.Id})
		if err != nil {
			return fmt.Errorf("r.UpdateSession: %w", err)
		}

		session := repoSessionToPBSession(repoSession)
		session.Participants = repoUsersToPBUsers(repoParticipants...)
		session.Winner = winnerUser

		err = h.extractor.StoreExtraction(ctx, session)
		if err != nil {
			h.logger.ErrorContext(ctx, "store extraction failed", "session_id", req.Id, "winner_id", winnerUser.Id, "error", err)
			return connect.NewError(connect.CodeInternal, err)
		}

		return nil
	})
	if err != nil {
		h.logger.WarnContext(ctx, "end session failed", "session_id", req.Id, "error", err)
		return nil, repoErrorToAPIError(err)
	}

	h.logger.InfoContext(ctx, "session ended", "session_id", req.Id, "winner_id", winnerUser.Id, "participants", len(repoParticipants))
	return &pb.EndSessionResponse{Winner: winnerUser}, nil
}

// SetSessionMovie set the movie id watched in a session
//
// watched movie id can be set only for closed sessions and can be called only by the winner of the session
// the movie watched can be changed by the winner
// SetSessionMovie records the watched movie for a closed session.
func (h *Handler) SetSessionMovie(ctx context.Context, req *pb.SetSessionMovieRequest) (*pb.SetSessionMovieResponse, error) {
	authSession, ok := authn.GetInfo(ctx).(auth.Session)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing auth session"))
	}

	if req.SessionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("session_id is empty"))
	}

	if req.MovieId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("movie_id is null"))
	}

	err := h.repository.WithTx(ctx, func(ctx context.Context, s persistence.Storage) error {
		session, err := s.GetSession(ctx, persistence.GetSessionArg{ID: req.SessionId})
		if err != nil {
			slog.ErrorContext(ctx, "s.GetSession", "error", err)
			return fmt.Errorf("getting session: %w", err)
		}

		if session.ClosedAt.IsZero() {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("session is not closed"))
		}

		if session.WinnerID == "" {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("session has no winner"))
		}

		if session.WinnerID != authSession.UserId {
			return connect.NewError(connect.CodePermissionDenied, errors.New("calling user is not the winner"))
		}

		if session.WatchedMovieID == req.MovieId {
			return nil
		}

		movie, err := s.GetMovie(ctx, persistence.GetMovieArg{MovieID: req.MovieId})
		if err != nil {
			return fmt.Errorf("getting movie: %w", err)
		}

		if movie.UserID != authSession.UserId {
			return connect.NewError(connect.CodePermissionDenied, errors.New("movie does not belong to calling user"))
		}

		if movie.Status == persistence.MovieStatusWatched {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("movie already watched"))
		}

		if session.WatchedMovieID != "" {
			status := persistence.MovieStatusPending
			if _, err = s.UpdateMovie(ctx, persistence.UpdateMovieArg{ID: session.WatchedMovieID, Status: &status}); err != nil {
				return fmt.Errorf("s.UpdateMovie(previous watched movie): %w", err)
			}
		}

		if _, err := s.UpdateSession(ctx, persistence.UpdateSessionArg{ID: session.ID, WatchedMovieID: &req.MovieId}); err != nil {
			return fmt.Errorf("s.UpdateSession: %w", err)
		}

		status := persistence.MovieStatusWatched
		if _, err = s.UpdateMovie(ctx, persistence.UpdateMovieArg{ID: req.MovieId, Status: &status}); err != nil {
			return fmt.Errorf("s.UpdateMovie(selected movie): %w", err)
		}
		return nil
	})
	if err != nil {
		h.logger.ErrorContext(ctx, "h.repository.WithTx failed", "error", err)
		return nil, repoErrorToAPIError(err)
	}

	return &pb.SetSessionMovieResponse{}, nil
}

// DeleteSession deletes a movie selection session.
func (h *Handler) DeleteSession(ctx context.Context, req *pb.DeleteSessionRequest) (*pb.DeleteSessionResponse, error) {
	if req.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is empty"))
	}

	session, err := h.repository.DeleteSession(ctx, req.Id)
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	h.logger.InfoContext(ctx, "session deleted", "session_id", session.ID)
	return &pb.DeleteSessionResponse{Session: repoSessionToPBSession(session)}, nil
}

// AddParticipant adds a user to a movie selection session.
func (h *Handler) AddParticipant(ctx context.Context, req *pb.AddParticipantRequest) (*pb.AddParticipantResponse, error) {
	if req.SessionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("session_id is empty"))
	}

	if req.UserId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is empty"))
	}

	_, err := h.repository.CreateParticipant(ctx, persistence.CreateParticipantArg{SessionID: req.SessionId, UserID: req.UserId})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	h.logger.InfoContext(ctx, "participant added", "session_id", req.SessionId, "user_id", req.UserId)
	return &pb.AddParticipantResponse{}, nil
}

// RemoveParticipant removes a user from a movie selection session.
func (h *Handler) RemoveParticipant(ctx context.Context, req *pb.RemoveParticipantRequest) (*pb.RemoveParticipantResponse, error) {
	if req.SessionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("session_id is empty"))
	}

	if req.UserId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is empty"))
	}

	err := h.repository.DeleteParticipant(ctx, persistence.DeleteParticipantArg{SessionID: req.SessionId, UserID: req.UserId})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	h.logger.InfoContext(ctx, "participant removed", "session_id", req.SessionId, "user_id", req.UserId)
	return &pb.RemoveParticipantResponse{}, nil
}

// ListParticipants lists users in a movie selection session.
func (h *Handler) ListParticipants(ctx context.Context, req *pb.ListParticipantsRequest) (*pb.ListParticipantsResponse, error) {
	if req.SessionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("session_id is empty"))
	}

	listRet, err := h.repository.ListParticipants(ctx, persistence.ListParticipantsArg{SessionID: req.SessionId})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	users := repoUsersToPBUsers(listRet.Participants...)
	return &pb.ListParticipantsResponse{Participants: users}, nil
}

// AddUserMovie adds a movie to the current user's list.
func (h *Handler) AddUserMovie(ctx context.Context, req *pb.AddUserMovieRequest) (*pb.AddUserMovieResponse, error) {
	session, ok := authn.GetInfo(ctx).(auth.Session)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing session"))
	}

	if req.MovieTitle == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("movie_title is null"))
	}

	movie, err := h.repository.CreateMovie(ctx, persistence.CreateMovieArg{UserID: session.UserId, MovieTitle: req.MovieTitle})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	h.logger.InfoContext(ctx, "movie added", "user_id", session.UserId, "movie_title", movie.Title)
	return &pb.AddUserMovieResponse{Movie: &pb.Movie{Id: movie.ID, Title: movie.Title}}, nil
}

// ListUserMovies lists movies for a user.
func (h *Handler) ListUserMovies(ctx context.Context, req *pb.ListUserMoviesRequest) (*pb.ListUserMoviesResponse, error) {
	if req.UserId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is null"))
	}

	listMoviesRet, err := h.repository.ListMovies(ctx, persistence.ListMoviesArg{UserID: req.UserId})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	return &pb.ListUserMoviesResponse{Movies: repoMoviesToPB(listMoviesRet.Movies...)}, nil
}

// DeleteUserMovie deletes a movie from the current user's list.
func (h *Handler) DeleteUserMovie(ctx context.Context, req *pb.DeleteUserMovieRequest) (*pb.DeleteUserMovieResponse, error) {
	if req.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is empty"))
	}

	session, ok := authn.GetInfo(ctx).(auth.Session)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing session"))
	}

	_, err := h.repository.DeleteMovie(ctx, persistence.DeleteMovieArg{UserID: session.UserId, MovieID: req.Id})
	if err != nil {
		return nil, repoErrorToAPIError(err)
	}

	h.logger.InfoContext(ctx, "movie deleted", "user_id", session.UserId, "movie_title", req.Id)
	return &pb.DeleteUserMovieResponse{}, nil
}
