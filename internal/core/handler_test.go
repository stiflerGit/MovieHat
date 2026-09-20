package v1

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/auth"
	"github.com/stiflerGit/moviehat/internal/core/mocks"
	extractormock "github.com/stiflerGit/moviehat/internal/core/mocks"
	"github.com/stiflerGit/moviehat/internal/core/persistence"
	persistencemock "github.com/stiflerGit/moviehat/internal/core/persistence/mocks"
	"google.golang.org/protobuf/types/known/timestamppb"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
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
	extractor := extractormock.NewMockExtractor(ctrl)
	h := New(store, extractor, nil, WithLogger(testLogger()))

	store.EXPECT().CreateUser(gomock.Any(), persistence.CreateUserArg{UserID: "auth-u1"}).
		Return(persistence.User{ID: "auth-u1", Name: "john"}, nil)

	resp, err := h.CreateUser(t.Context(), CreateUserRequest{UserID: "auth-u1"})
	require.NoError(t, err)
	require.Equal(t, "auth-u1", resp.User.Id)
}

func TestHandlerUpdateUser_MissingSession(t *testing.T) {
	h := New(nil, nil, nil, WithLogger(testLogger()))
	_, err := h.UpdateUser(t.Context(), &pb.UpdateUserRequest{Name: "john"})
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestMovieHandlerAddUserMovie(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := persistencemock.NewMockTransactionalStorage(ctrl)
	movieSearchEngine := mocks.NewMockMovieSearchEngine(ctrl)

	h := New(store, nil, movieSearchEngine, WithLogger(testLogger()))

	ctx := authn.SetInfo(t.Context(), auth.Session{UserId: "user-1"})
	store.EXPECT().AddMovie(gomock.Any(), persistence.AddMovieArg{UserID: "user-1", MovieID: "12345"}).
		Return(persistence.Movie{ID: "movie-1", Title: "Alien"}, nil)
	movieSearchEngine.EXPECT().GetByID(ctx, "12345").Return(
		&pb.Movie{
			Id:          "12345",
			Title:       "Alien",
			ReleaseDate: timestamppb.New(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)),
			PosterPath:  "/alien.jpg",
		}, nil)

	resp, err := h.AddUserMovie(ctx, &pb.AddUserMovieRequest{MovieId: "12345"})
	require.NoError(t, err)
	require.Equal(t, "movie-1", resp.Movie.Id)
	require.Equal(t, "Alien", resp.Movie.Title)
}

func TestHandlerListUserMovies_Validation(t *testing.T) {
	h := New(nil, nil, nil, WithLogger(testLogger()))

	_, err := h.ListUserMovies(t.Context(), &pb.ListUserMoviesRequest{})
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestHandlerDeleteUserMovie_MissingSession(t *testing.T) {
	h := New(nil, nil, nil, WithLogger(testLogger()))

	_, err := h.DeleteUserMovie(t.Context(), &pb.DeleteUserMovieRequest{Id: "m-1"})
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestHandlerEndSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := persistencemock.NewMockTransactionalStorage(ctrl)
	extractor := extractormock.NewMockExtractor(ctrl)
	h := New(store, extractor, nil, WithLogger(testLogger()))

	participants := []persistence.User{{ID: "u1", Name: "john"}, {ID: "u2", Name: "jane"}}
	winner := &pb.User{Id: "u2", Name: "jane"}

	store.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, persistence.Storage) error) error {
			return fn(ctx, store)
		},
	)
	store.EXPECT().CloseSession(gomock.Any(), "session-1").Return(nil)
	store.EXPECT().ListParticipants(gomock.Any(), persistence.ListParticipantsArg{SessionID: "session-1"}).
		Return(persistence.ListParticipantsRet{Participants: participants}, nil)
	extractor.EXPECT().Extract(gomock.Any(), gomock.Any()).Return(winner, nil)
	store.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req persistence.UpdateSessionArg) (persistence.Session, error) {
			require.NotNil(t, req.WinnerID)
			require.Equal(t, "u2", *req.WinnerID)
			return persistence.Session{ID: "session-1", CreatedAt: time.Unix(1, 0), ClosedAt: time.Now(), WinnerID: "u2"}, nil
		},
	)
	extractor.EXPECT().StoreExtraction(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := h.EndSession(t.Context(), &pb.EndSessionRequest{Id: "session-1"})
	require.NoError(t, err)
	require.Equal(t, "u2", resp.Winner.Id)
}

func TestHandlerEndSession_StoreExtractionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := persistencemock.NewMockTransactionalStorage(ctrl)
	extractor := extractormock.NewMockExtractor(ctrl)
	h := New(store, extractor, nil, WithLogger(testLogger()))

	participants := []persistence.User{{ID: "u1", Name: "john"}, {ID: "u2", Name: "jack"}}
	winner := &pb.User{Id: "u1", Name: "john"}

	store.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, persistence.Storage) error) error {
			return fn(ctx, store)
		},
	)
	store.EXPECT().CloseSession(gomock.Any(), "session-1").Return(nil)
	store.EXPECT().ListParticipants(gomock.Any(), persistence.ListParticipantsArg{SessionID: "session-1"}).
		Return(persistence.ListParticipantsRet{Participants: participants}, nil)
	extractor.EXPECT().Extract(gomock.Any(), gomock.Any()).Return(winner, nil)
	store.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).Return(persistence.Session{ID: "session-1", WinnerID: "u1"}, nil)
	extractor.EXPECT().StoreExtraction(gomock.Any(), gomock.Any()).Return(errors.New("boom"))

	_, err := h.EndSession(t.Context(), &pb.EndSessionRequest{Id: "session-1"})
	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestHandlerSetSessionMovie(t *testing.T) {
	type testCase struct {
		name      string
		ctx       func(t *testing.T) context.Context
		req       *pb.SetSessionMovieRequest
		setupMock func(store *persistencemock.MockTransactionalStorage)
		wantCode  connect.Code
	}

	authCtx := func(userID string) func(*testing.T) context.Context {
		return func(t *testing.T) context.Context {
			return authn.SetInfo(t.Context(), auth.Session{UserId: userID})
		}
	}

	const (
		sessionID  = "session-1"
		oldMovieID = "movie-old"
		newMovieID = "movie-new"
	)

	testCases := []testCase{
		{
			name:     "missing auth session",
			ctx:      func(t *testing.T) context.Context { return t.Context() },
			req:      &pb.SetSessionMovieRequest{SessionId: sessionID, MovieId: newMovieID},
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name:     "empty session_id",
			ctx:      authCtx("winner-1"),
			req:      &pb.SetSessionMovieRequest{MovieId: newMovieID},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name:     "empty movie_id",
			ctx:      authCtx("winner-1"),
			req:      &pb.SetSessionMovieRequest{SessionId: sessionID},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name: "session is not closed",
			ctx:  authCtx("winner-1"),
			req:  &pb.SetSessionMovieRequest{SessionId: sessionID, MovieId: newMovieID},
			setupMock: func(store *persistencemock.MockTransactionalStorage) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).Return(
					persistence.Session{ID: sessionID, WinnerID: "winner-1"}, nil,
				)
			},
			wantCode: connect.CodeFailedPrecondition,
		},
		{
			name: "calling user is not winner",
			ctx:  authCtx("winner-1"),
			req:  &pb.SetSessionMovieRequest{SessionId: sessionID, MovieId: newMovieID},
			setupMock: func(store *persistencemock.MockTransactionalStorage) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).Return(
					persistence.Session{ID: sessionID, ClosedAt: time.Now(), WinnerID: "winner-2"}, nil,
				)
			},
			wantCode: connect.CodePermissionDenied,
		},
		{
			name: "movie does not belong to winner",
			ctx:  authCtx("winner-1"),
			req:  &pb.SetSessionMovieRequest{SessionId: sessionID, MovieId: newMovieID},
			setupMock: func(store *persistencemock.MockTransactionalStorage) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).Return(
					persistence.Session{ID: sessionID, ClosedAt: time.Now(), WinnerID: "winner-1"}, nil,
				)
				store.EXPECT().GetMovie(gomock.Any(), persistence.GetMovieArg{MovieID: newMovieID}).Return(
					persistence.Movie{ID: newMovieID, UserID: "winner-2", Status: persistence.MovieStatusPending}, nil,
				)
			},
			wantCode: connect.CodePermissionDenied,
		},
		{
			name: "movie already watched",
			ctx:  authCtx("winner-1"),
			req:  &pb.SetSessionMovieRequest{SessionId: sessionID, MovieId: newMovieID},
			setupMock: func(store *persistencemock.MockTransactionalStorage) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).Return(
					persistence.Session{ID: sessionID, ClosedAt: time.Now(), WinnerID: "winner-1"}, nil,
				)
				store.EXPECT().GetMovie(gomock.Any(), persistence.GetMovieArg{MovieID: newMovieID}).Return(
					persistence.Movie{ID: newMovieID, UserID: "winner-1", Status: persistence.MovieStatusWatched}, nil,
				)
			},
			wantCode: connect.CodeFailedPrecondition,
		},
		{
			name: "idempotent when session already has same movie",
			ctx:  authCtx("winner-1"),
			req:  &pb.SetSessionMovieRequest{SessionId: sessionID, MovieId: newMovieID},
			setupMock: func(store *persistencemock.MockTransactionalStorage) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).Return(
					persistence.Session{ID: sessionID, ClosedAt: time.Now(), WinnerID: "winner-1", WatchedMovieID: newMovieID}, nil,
				)
			},
		},
		{
			name: "success first assignment",
			ctx:  authCtx("winner-1"),
			req:  &pb.SetSessionMovieRequest{SessionId: sessionID, MovieId: newMovieID},
			setupMock: func(store *persistencemock.MockTransactionalStorage) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).Return(
					persistence.Session{ID: sessionID, ClosedAt: time.Now(), WinnerID: "winner-1"}, nil,
				)
				store.EXPECT().GetMovie(gomock.Any(), persistence.GetMovieArg{MovieID: newMovieID}).Return(
					persistence.Movie{ID: newMovieID, UserID: "winner-1", Status: persistence.MovieStatusPending}, nil,
				)
				gomock.InOrder(
					store.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).DoAndReturn(
						func(_ context.Context, arg persistence.UpdateSessionArg) (persistence.Session, error) {
							require.NotNil(t, arg.WatchedMovieID)
							require.Equal(t, newMovieID, *arg.WatchedMovieID)
							return persistence.Session{ID: sessionID}, nil
						},
					),
					store.EXPECT().UpdateMovie(gomock.Any(), gomock.Any()).DoAndReturn(
						func(_ context.Context, arg persistence.UpdateMovieArg) (persistence.Movie, error) {
							require.NotNil(t, arg.Status)
							require.Equal(t, persistence.MovieStatusWatched, *arg.Status)
							return persistence.Movie{ID: arg.ID}, nil
						},
					),
				)
			},
		},
		{
			name: "success replacing previously selected movie",
			ctx:  authCtx("winner-1"),
			req:  &pb.SetSessionMovieRequest{SessionId: sessionID, MovieId: newMovieID},
			setupMock: func(store *persistencemock.MockTransactionalStorage) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).Return(
					persistence.Session{ID: sessionID, ClosedAt: time.Now(), WinnerID: "winner-1", WatchedMovieID: oldMovieID}, nil,
				)
				store.EXPECT().GetMovie(gomock.Any(), persistence.GetMovieArg{MovieID: newMovieID}).Return(
					persistence.Movie{ID: newMovieID, UserID: "winner-1", Status: persistence.MovieStatusPending}, nil,
				)
				gomock.InOrder(
					store.EXPECT().UpdateMovie(gomock.Any(), gomock.Any()).DoAndReturn(
						func(_ context.Context, arg persistence.UpdateMovieArg) (persistence.Movie, error) {
							require.Equal(t, oldMovieID, arg.ID)
							require.NotNil(t, arg.Status)
							require.Equal(t, persistence.MovieStatusPending, *arg.Status)
							return persistence.Movie{ID: arg.ID}, nil
						},
					),
					store.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).DoAndReturn(
						func(_ context.Context, arg persistence.UpdateSessionArg) (persistence.Session, error) {
							require.NotNil(t, arg.WatchedMovieID)
							require.Equal(t, newMovieID, *arg.WatchedMovieID)
							return persistence.Session{ID: sessionID}, nil
						},
					),
					store.EXPECT().UpdateMovie(gomock.Any(), gomock.Any()).DoAndReturn(
						func(_ context.Context, arg persistence.UpdateMovieArg) (persistence.Movie, error) {
							require.Equal(t, newMovieID, arg.ID)
							require.NotNil(t, arg.Status)
							require.Equal(t, persistence.MovieStatusWatched, *arg.Status)
							return persistence.Movie{ID: arg.ID}, nil
						},
					),
				)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := persistencemock.NewMockTransactionalStorage(ctrl)
			h := New(store, nil, nil, WithLogger(testLogger()))

			if tc.setupMock != nil {
				store.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context, persistence.Storage) error) error {
						return fn(ctx, store)
					},
				)
				tc.setupMock(store)
			}

			resp, err := h.SetSessionMovie(tc.ctx(t), tc.req)
			if tc.wantCode == 0 {
				require.NoError(t, err)
				require.NotNil(t, resp)
				return
			}

			require.Error(t, err)
			require.Equal(t, tc.wantCode, connect.CodeOf(err))
		})
	}
}

func TestHandlerGetSessionProbabilities(t *testing.T) {
	const sessionID = "session-1"

	type testCase struct {
		name              string
		req               *pb.GetSessionProbabilitiesRequest
		setupMock         func(store *persistencemock.MockTransactionalStorage, extractor *extractormock.MockExtractor)
		wantCode          connect.Code // 0 => success
		wantProbabilities []*pb.GetSessionProbabilitiesResponse_ParticipantProbabilities
	}

	probability := func(id string, p float64) *pb.GetSessionProbabilitiesResponse_ParticipantProbabilities {
		return &pb.GetSessionProbabilitiesResponse_ParticipantProbabilities{UserId: id, Probability: p}
	}

	testCases := []testCase{
		{
			name:     "empty session_id fails validation",
			req:      &pb.GetSessionProbabilitiesRequest{},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name: "session not found",
			req:  &pb.GetSessionProbabilitiesRequest{SessionId: sessionID},
			setupMock: func(store *persistencemock.MockTransactionalStorage, _ *extractormock.MockExtractor) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).
					Return(persistence.Session{}, persistence.ErrNotFound)
			},
			wantCode: connect.CodeNotFound,
		},
		{
			name: "get session internal error",
			req:  &pb.GetSessionProbabilitiesRequest{SessionId: sessionID},
			setupMock: func(store *persistencemock.MockTransactionalStorage, _ *extractormock.MockExtractor) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).
					Return(persistence.Session{}, errors.New("boom"))
			},
			wantCode: connect.CodeInternal,
		},
		{
			name: "list participants error",
			req:  &pb.GetSessionProbabilitiesRequest{SessionId: sessionID},
			setupMock: func(store *persistencemock.MockTransactionalStorage, _ *extractormock.MockExtractor) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).
					Return(persistence.Session{ID: sessionID}, nil)
				store.EXPECT().ListParticipants(gomock.Any(), persistence.ListParticipantsArg{SessionID: sessionID}).
					Return(persistence.ListParticipantsRet{}, errors.New("boom"))
			},
			wantCode: connect.CodeInternal,
		},
		{
			name: "no participants returns empty probabilities",
			req:  &pb.GetSessionProbabilitiesRequest{SessionId: sessionID},
			setupMock: func(store *persistencemock.MockTransactionalStorage, _ *extractormock.MockExtractor) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).
					Return(persistence.Session{ID: sessionID}, nil)
				store.EXPECT().ListParticipants(gomock.Any(), persistence.ListParticipantsArg{SessionID: sessionID}).
					Return(persistence.ListParticipantsRet{}, nil)
			},
			wantProbabilities: nil,
		},
		{
			name: "single participant is certain and skips extractor",
			req:  &pb.GetSessionProbabilitiesRequest{SessionId: sessionID},
			setupMock: func(store *persistencemock.MockTransactionalStorage, _ *extractormock.MockExtractor) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).
					Return(persistence.Session{ID: sessionID}, nil)
				store.EXPECT().ListParticipants(gomock.Any(), persistence.ListParticipantsArg{SessionID: sessionID}).
					Return(persistence.ListParticipantsRet{Participants: []persistence.User{{ID: "u1"}}}, nil)
			},
			wantProbabilities: []*pb.GetSessionProbabilitiesResponse_ParticipantProbabilities{probability("u1", 1.0)},
		},
		{
			name: "multi participants map probabilities by position",
			req:  &pb.GetSessionProbabilitiesRequest{SessionId: sessionID},
			setupMock: func(store *persistencemock.MockTransactionalStorage, extractor *extractormock.MockExtractor) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).
					Return(persistence.Session{ID: sessionID}, nil)
				store.EXPECT().ListParticipants(gomock.Any(), persistence.ListParticipantsArg{SessionID: sessionID}).
					Return(persistence.ListParticipantsRet{Participants: []persistence.User{{ID: "u1"}, {ID: "u2"}, {ID: "u3"}}}, nil)
				extractor.EXPECT().GetProbabilities(gomock.Any(), gomock.Any()).
					Return([]float64{0.2, 0.3, 0.5}, nil)
			},
			wantProbabilities: []*pb.GetSessionProbabilitiesResponse_ParticipantProbabilities{
				probability("u1", 0.2), probability("u2", 0.3), probability("u3", 0.5),
			},
		},
		{
			name: "extractor error",
			req:  &pb.GetSessionProbabilitiesRequest{SessionId: sessionID},
			setupMock: func(store *persistencemock.MockTransactionalStorage, extractor *extractormock.MockExtractor) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).
					Return(persistence.Session{ID: sessionID}, nil)
				store.EXPECT().ListParticipants(gomock.Any(), persistence.ListParticipantsArg{SessionID: sessionID}).
					Return(persistence.ListParticipantsRet{Participants: []persistence.User{{ID: "u1"}, {ID: "u2"}}}, nil)
				extractor.EXPECT().GetProbabilities(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("boom"))
			},
			wantCode: connect.CodeInternal,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := persistencemock.NewMockTransactionalStorage(ctrl)
			extractor := extractormock.NewMockExtractor(ctrl)
			h := New(store, extractor, nil, WithLogger(testLogger()))

			if tc.setupMock != nil {
				tc.setupMock(store, extractor)
			}

			resp, err := h.GetSessionProbabilities(t.Context(), tc.req)
			if tc.wantCode != 0 {
				require.Error(t, err)
				require.Equal(t, tc.wantCode, connect.CodeOf(err))
				return
			}

			require.NoError(t, err)
			require.Len(t, resp.ParticipantsProbabilities, len(tc.wantProbabilities))
			for i, want := range tc.wantProbabilities {
				require.Equal(t, want.UserId, resp.ParticipantsProbabilities[i].UserId)
				require.InDelta(t, want.Probability, resp.ParticipantsProbabilities[i].Probability, 1e-9)
			}
		})
	}
}
