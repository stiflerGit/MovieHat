package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/lottery"
	"github.com/stiflerGit/moviehat/internal/persistence"
)

//go:generate mockgen -package mocks -destination mocks/mocks.go -source=handler.go

// Storage stores Session releated entities.
type Storage interface {
	persistence.Transactor
	persistence.SessionsStorage
	persistence.ParticipantsStorage
}

// Lottery selects and records winners for closed sessions.
type Lottery interface {
	lottery.Drawer
	lottery.DrawStorage
}

// Handler provides MovieHat user, movie, and session operations.
type Handler struct {
	repository Storage
	lottery    Lottery
	logger     *slog.Logger
}

// New creates a MovieHat handler.
func New(
	repository Storage,
	extractionHandler Lottery,
	options ...Option,
) *Handler {
	h := &Handler{
		repository: repository,
		lottery:    extractionHandler,
		logger:     slog.Default().With("component", "session"),
	}

	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(h)
	}

	return h
}

// CreateSession creates a movie selection session.
func (h *Handler) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
	session, err := h.repository.CreateSession(ctx, persistence.CreateSessionArg{})
	if err != nil {
		return nil, fmt.Errorf("h.repository.CreateSession: %w", err)
	}

	h.logger.InfoContext(ctx, "session created", "session_id", session.ID)
	return &pb.CreateSessionResponse{Session: repoSessionToPBSession(session)}, nil
}

// ListSessions lists movie selection sessions.
func (h *Handler) ListSessions(ctx context.Context, req *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	repoSessions, err := h.repository.ListSessions(ctx, persistence.ListSessionsAg{})
	if err != nil {
		return nil, fmt.Errorf("h.repository.ListSessions: %w", err)
	}

	return &pb.ListSessionsResponse{Sessions: repoSessionsToPB(repoSessions.Sessions...)}, nil
}

// GetSession returns a movie selection session.
func (h *Handler) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.GetSessionResponse, error) {
	session, err := h.repository.GetSession(ctx, persistence.GetSessionArg{ID: req.Id})
	if err != nil {
		return nil, fmt.Errorf("h.repository.GetSession: %w", err)
	}

	participants, err := h.repository.ListParticipants(ctx, persistence.ListParticipantsArg{SessionID: req.Id})
	if err != nil {
		return nil, fmt.Errorf("h.repository.ListParticipants: %w", err)
	}

	pbSession := repoSessionToPBSession(session)
	pbSession.Participants = repoUsersToPBUsers(participants.Participants...)
	return &pb.GetSessionResponse{Session: pbSession}, nil
}

// EndSession ends a session, extracting a winner among participants of the session and store results
func (h *Handler) EndSession(ctx context.Context, req *pb.EndSessionRequest) (*pb.EndSessionResponse, error) {
	var winnerUser *pb.User

	var repoSession persistence.Session
	var repoParticipants []persistence.User
	var session *pb.Session
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
			// not enough participant for the session. Cannot be closed, just deleted
			return ErrFailedPrecondition{fmt.Errorf("not enough participants: count=%d", len(listParticipantsRet.Participants))}
		}

		repoParticipants = listParticipantsRet.Participants
		pbSession := &pb.Session{Participants: repoUsersToPBUsers(repoParticipants...)}
		sessionWrapper := SessionWrapper{pbSession}
		participantIDs := sessionWrapper.ParticipantIDs()

		drawRet, err := h.lottery.Draw(ctx, lottery.DrawArg{UserIDs: participantIDs})
		if err != nil {
			return fmt.Errorf("h.lottery.Draw: %w", err)
		}
		winnerUser = pbSession.Participants[drawRet.Index]

		repoSession, err = s.UpdateSession(ctx, persistence.UpdateSessionArg{ID: req.Id, WinnerID: &winnerUser.Id})
		if err != nil {
			return fmt.Errorf("r.UpdateSession: %w", err)
		}

		session = repoSessionToPBSession(repoSession)
		session.Participants = repoUsersToPBUsers(repoParticipants...)
		session.Winner = winnerUser

		_, err = h.lottery.StoreDrawResult(ctx, lottery.StoreDrawResultArg{UserIDs: participantIDs, WinnerID: winnerUser.Id})
		if err != nil {
			h.logger.ErrorContext(ctx, "store extraction failed", "session_id", req.Id, "winner_id", winnerUser.Id, "error", err)
			return fmt.Errorf("h.extractor.StoreExtraction: %w", err)
		}

		return nil
	})
	if err != nil {
		h.logger.WarnContext(ctx, "end session failed", "session_id", req.Id, "error", err)
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	h.logger.InfoContext(ctx, "session ended", "session_id", req.Id, "winner_id", winnerUser.Id, "participants", len(repoParticipants))
	return &pb.EndSessionResponse{Winner: winnerUser}, nil
}

type SetSessionMovieRequest struct {
	UserID string
	*pb.SetSessionMovieRequest
}

// SetSessionMovie set the movie id watched in a session
//
// watched movie id can be set only for closed sessions and can be called only by the winner of the session
// the movie watched can be changed by the winner
// SetSessionMovie records the watched movie for a closed session.
func (h *Handler) SetSessionMovie(ctx context.Context, req SetSessionMovieRequest) (*pb.SetSessionMovieResponse, error) {
	err := h.repository.WithTx(ctx, func(ctx context.Context, s persistence.Storage) error {
		session, err := s.GetSession(ctx, persistence.GetSessionArg{ID: req.SessionId})
		if err != nil {
			slog.ErrorContext(ctx, "s.GetSession", "error", err)
			return fmt.Errorf("getting session: %w", err)
		}

		if session.ClosedAt.IsZero() {
			return ErrFailedPrecondition{errors.New("session is not closed")}
		}

		if session.WinnerID == "" {
			return ErrFailedPrecondition{errors.New("session has no winner")}
		}

		if session.WinnerID != req.UserID {
			return ErrFailedPrecondition{errors.New("calling user is not the winner")}
		}

		if session.WatchedMovieID == req.MovieId {
			return nil
		}

		movie, err := s.GetMovie(ctx, persistence.GetMovieArg{UserID: req.UserID, MovieID: req.MovieId})
		if err != nil {
			return fmt.Errorf("getting movie: %w", err)
		}

		if movie.Status == persistence.MovieStatusWatched {
			return ErrFailedPrecondition{errors.New("movie already watched")}
		}

		if session.WatchedMovieID != "" {
			status := persistence.MovieStatusPending
			if err = s.UpdateMovies(ctx, persistence.UpdateMoviesArg{ID: session.WatchedMovieID, Status: &status}); err != nil {
				return fmt.Errorf("s.UpdateMovie(previous watched movie): %w", err)
			}
		}

		if _, err := s.UpdateSession(ctx, persistence.UpdateSessionArg{ID: session.ID, WatchedMovieID: &req.MovieId}); err != nil {
			return fmt.Errorf("s.UpdateSession: %w", err)
		}

		status := persistence.MovieStatusWatched
		if err = s.UpdateMovies(ctx, persistence.UpdateMoviesArg{ID: req.MovieId, Status: &status}); err != nil {
			return fmt.Errorf("s.UpdateMovies(selected movie): %w", err)
		}
		return nil
	})
	if err != nil {
		h.logger.ErrorContext(ctx, "h.repository.WithTx failed", "error", err)
		return nil, fmt.Errorf("h.repository.WithTx: %w", err)
	}

	return &pb.SetSessionMovieResponse{}, nil
}

// returns the probability of each participant to be extracted for a given session.
func (h *Handler) GetSessionProbabilities(ctx context.Context, req *pb.GetSessionProbabilitiesRequest) (*pb.GetSessionProbabilitiesResponse, error) {
	session, err := h.repository.GetSession(ctx, persistence.GetSessionArg{ID: req.SessionId})
	if err != nil {
		if !errors.Is(err, persistence.ErrNotFound) {
			h.logger.ErrorContext(ctx, "GetSessionProbabilities h.repository.GetSession", "error", err)
		}
		return nil, fmt.Errorf("h.repository.GetSession: %w", err)
	}
	pbSession := repoSessionToPBSession(session)

	participants, err := h.repository.ListParticipants(ctx, persistence.ListParticipantsArg{SessionID: req.SessionId})
	if err != nil {
		h.logger.ErrorContext(ctx, "GetSessionProbabilities h.repository.ListParticipants", "error", err)
		return nil, fmt.Errorf("h.repository.ListParticipants: %w", err)
	}
	pbSession.Participants = repoUsersToPBUsers(participants.Participants...)

	if len(pbSession.Participants) == 0 {
		return &pb.GetSessionProbabilitiesResponse{}, nil
	}

	if len(pbSession.Participants) == 1 {
		return &pb.GetSessionProbabilitiesResponse{
			ParticipantsProbabilities: []*pb.GetSessionProbabilitiesResponse_ParticipantProbabilities{
				{
					UserId:      pbSession.Participants[0].Id,
					Probability: 1.0,
				},
			},
		}, nil
	}

	sessionWrapper := SessionWrapper{pbSession}
	arg := lottery.GetProbabilitiesArg{UserIDs: sessionWrapper.ParticipantIDs()}
	getProbabilitiesRet, err := h.lottery.GetProbabilities(ctx, lottery.GetProbabilitiesArg{UserIDs: sessionWrapper.ParticipantIDs()})
	if err != nil {
		return nil, fmt.Errorf("h.lottery.GetProbabilities: %v", err)
	}

	slog.Info("h.lottery.GetProbabilities returned", "arg", arg, "ret", getProbabilitiesRet)
	if len(getProbabilitiesRet.Probabilities) != len(pbSession.Participants) {
		return nil, ErrInternal{errors.New("len(probabilities) != len(pbSession.Participants)")}
	}

	participantProbabilities := make([]*pb.GetSessionProbabilitiesResponse_ParticipantProbabilities, 0, len(pbSession.Participants))
	for i, participant := range pbSession.Participants {
		probability := getProbabilitiesRet.Probabilities[i]
		participantProbabilities = append(participantProbabilities,
			&pb.GetSessionProbabilitiesResponse_ParticipantProbabilities{
				UserId:      participant.Id,
				Probability: probability,
			},
		)
	}

	return &pb.GetSessionProbabilitiesResponse{ParticipantsProbabilities: participantProbabilities}, nil
}

// DeleteSession deletes a movie selection session.
func (h *Handler) DeleteSession(ctx context.Context, req *pb.DeleteSessionRequest) (*pb.DeleteSessionResponse, error) {
	session, err := h.repository.DeleteSession(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("h.repository.DeleteSession: %w", err)
	}

	h.logger.InfoContext(ctx, "session deleted", "session_id", session.ID)
	return &pb.DeleteSessionResponse{Session: repoSessionToPBSession(session)}, nil
}

// AddParticipant adds a user to a movie selection session.
func (h *Handler) AddParticipant(ctx context.Context, req *pb.AddParticipantRequest) (*pb.AddParticipantResponse, error) {
	_, err := h.repository.CreateParticipant(ctx, persistence.CreateParticipantArg{SessionID: req.SessionId, UserID: req.UserId})
	if err != nil {
		if errors.Is(err, persistence.ErrAlreadyExists) {
			return &pb.AddParticipantResponse{}, nil
		}
		return nil, fmt.Errorf("h.repository.CreateParticipant: %w", err)
	}

	h.logger.InfoContext(ctx, "participant added", "session_id", req.SessionId, "user_id", req.UserId)
	return &pb.AddParticipantResponse{}, nil
}

// RemoveParticipant removes a user from a movie selection session.
func (h *Handler) RemoveParticipant(ctx context.Context, req *pb.RemoveParticipantRequest) (*pb.RemoveParticipantResponse, error) {
	err := h.repository.DeleteParticipant(ctx, persistence.DeleteParticipantArg{SessionID: req.SessionId, UserID: req.UserId})
	if err != nil {
		return nil, fmt.Errorf("h.repository.DeleteParticipant: %w", err)
	}

	h.logger.InfoContext(ctx, "participant removed", "session_id", req.SessionId, "user_id", req.UserId)
	return &pb.RemoveParticipantResponse{}, nil
}

// ListParticipants lists users in a movie selection session.
func (h *Handler) ListParticipants(ctx context.Context, req *pb.ListParticipantsRequest) (*pb.ListParticipantsResponse, error) {
	listRet, err := h.repository.ListParticipants(ctx, persistence.ListParticipantsArg{SessionID: req.SessionId})
	if err != nil {
		return nil, fmt.Errorf("h.repository.ListParticipants: %w", err)
	}

	users := repoUsersToPBUsers(listRet.Participants...)
	return &pb.ListParticipantsResponse{Participants: users}, nil
}
