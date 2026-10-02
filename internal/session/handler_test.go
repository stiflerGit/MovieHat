package session

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/lottery"
	"github.com/stiflerGit/moviehat/internal/persistence"
	persistencemock "github.com/stiflerGit/moviehat/internal/persistence/mocks"
	"github.com/stiflerGit/moviehat/internal/session/mocks"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func expectTx(store *persistencemock.MockTransactionalStorage) {
	store.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, persistence.Storage) error) error {
			return fn(ctx, store)
		},
	)
}

func TestNew(t *testing.T) {
	h := New(nil, nil)
	require.NotNil(t, h)
	require.NotNil(t, h.logger)
}

func TestHandlerEndSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := persistencemock.NewMockTransactionalStorage(ctrl)
	lot := mocks.NewMockLottery(ctrl)
	h := New(store, lot, WithLogger(testLogger()))

	participants := []persistence.User{{ID: "u1", Name: "john"}, {ID: "u2", Name: "jane"}}

	expectTx(store)
	store.EXPECT().CloseSession(gomock.Any(), "session-1").Return(nil)
	store.EXPECT().ListParticipants(gomock.Any(), persistence.ListParticipantsArg{SessionID: "session-1"}).
		Return(persistence.ListParticipantsRet{Participants: participants}, nil)
	lot.EXPECT().Draw(gomock.Any(), lottery.DrawArg{UserIDs: []string{"u1", "u2"}}).
		Return(lottery.DrawRet{UserID: "u2", Index: 1}, nil)
	store.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req persistence.UpdateSessionArg) (persistence.Session, error) {
			require.NotNil(t, req.WinnerID)
			require.Equal(t, "u2", *req.WinnerID)
			return persistence.Session{ID: "session-1", CreatedAt: time.Unix(1, 0), ClosedAt: time.Now(), WinnerID: "u2"}, nil
		},
	)
	lot.EXPECT().StoreDrawResult(gomock.Any(), lottery.StoreDrawResultArg{UserIDs: []string{"u1", "u2"}, WinnerID: "u2"}).
		Return(lottery.StoreDrawResultRet{}, nil)

	resp, err := h.EndSession(t.Context(), &pb.EndSessionRequest{Id: "session-1"})
	require.NoError(t, err)
	require.Equal(t, "u2", resp.Winner.Id)
}

func TestHandlerEndSession_StoreDrawResultError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := persistencemock.NewMockTransactionalStorage(ctrl)
	lot := mocks.NewMockLottery(ctrl)
	h := New(store, lot, WithLogger(testLogger()))

	participants := []persistence.User{{ID: "u1", Name: "john"}, {ID: "u2", Name: "jack"}}

	expectTx(store)
	store.EXPECT().CloseSession(gomock.Any(), "session-1").Return(nil)
	store.EXPECT().ListParticipants(gomock.Any(), persistence.ListParticipantsArg{SessionID: "session-1"}).
		Return(persistence.ListParticipantsRet{Participants: participants}, nil)
	lot.EXPECT().Draw(gomock.Any(), gomock.Any()).Return(lottery.DrawRet{UserID: "u1", Index: 0}, nil)
	store.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).Return(persistence.Session{ID: "session-1", WinnerID: "u1"}, nil)
	lot.EXPECT().StoreDrawResult(gomock.Any(), gomock.Any()).Return(lottery.StoreDrawResultRet{}, errors.New("boom"))

	_, err := h.EndSession(t.Context(), &pb.EndSessionRequest{Id: "session-1"})
	require.Error(t, err)
	require.ErrorContains(t, err, "boom")
}

func TestHandlerEndSession_NotEnoughParticipants(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := persistencemock.NewMockTransactionalStorage(ctrl)
	lot := mocks.NewMockLottery(ctrl)
	h := New(store, lot, WithLogger(testLogger()))

	expectTx(store)
	store.EXPECT().CloseSession(gomock.Any(), "session-1").Return(nil)
	store.EXPECT().ListParticipants(gomock.Any(), persistence.ListParticipantsArg{SessionID: "session-1"}).
		Return(persistence.ListParticipantsRet{Participants: []persistence.User{{ID: "u1"}}}, nil)

	_, err := h.EndSession(t.Context(), &pb.EndSessionRequest{Id: "session-1"})
	var pErr ErrFailedPrecondition
	require.ErrorAs(t, err, &pErr)
}

func TestHandlerSetSessionMovie(t *testing.T) {
	type testCase struct {
		name      string
		req       SetSessionMovieRequest
		setupMock func(store *persistencemock.MockTransactionalStorage)
		wantErr   error // nil => success
	}

	const (
		sessionID  = "session-1"
		userID     = "winner-1"
		oldMovieID = "movie-old"
		newMovieID = "movie-new"
	)

	newReq := func(pbReq *pb.SetSessionMovieRequest) SetSessionMovieRequest {
		return SetSessionMovieRequest{UserID: userID, SetSessionMovieRequest: pbReq}
	}

	testCases := []testCase{
		{
			name: "session is not closed",
			req:  newReq(&pb.SetSessionMovieRequest{SessionId: sessionID, MovieId: newMovieID}),
			setupMock: func(store *persistencemock.MockTransactionalStorage) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).Return(
					persistence.Session{ID: sessionID, WinnerID: userID}, nil,
				)
			},
			wantErr: ErrFailedPrecondition{},
		},
		{
			name: "calling user is not winner",
			req:  newReq(&pb.SetSessionMovieRequest{SessionId: sessionID, MovieId: newMovieID}),
			setupMock: func(store *persistencemock.MockTransactionalStorage) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).Return(
					persistence.Session{ID: sessionID, ClosedAt: time.Now(), WinnerID: "winner-2"}, nil,
				)
			},
			wantErr: ErrFailedPrecondition{},
		},
		{
			name: "movie already watched",
			req:  newReq(&pb.SetSessionMovieRequest{SessionId: sessionID, MovieId: newMovieID}),
			setupMock: func(store *persistencemock.MockTransactionalStorage) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).Return(
					persistence.Session{ID: sessionID, ClosedAt: time.Now(), WinnerID: userID}, nil,
				)
				store.EXPECT().GetMovie(gomock.Any(), persistence.GetMovieArg{UserID: userID, MovieID: newMovieID}).Return(
					persistence.Movie{ID: newMovieID, UserID: userID, Status: persistence.MovieStatusWatched}, nil,
				)
			},
			wantErr: ErrFailedPrecondition{},
		},
		{
			name: "idempotent when session already has same movie",
			req:  newReq(&pb.SetSessionMovieRequest{SessionId: sessionID, MovieId: newMovieID}),
			setupMock: func(store *persistencemock.MockTransactionalStorage) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).Return(
					persistence.Session{ID: sessionID, ClosedAt: time.Now(), WinnerID: userID, WatchedMovieID: newMovieID}, nil,
				)
			},
		},
		{
			name: "success first assignment",
			req:  newReq(&pb.SetSessionMovieRequest{SessionId: sessionID, MovieId: newMovieID}),
			setupMock: func(store *persistencemock.MockTransactionalStorage) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).Return(
					persistence.Session{ID: sessionID, ClosedAt: time.Now(), WinnerID: userID}, nil,
				)
				store.EXPECT().GetMovie(gomock.Any(), persistence.GetMovieArg{UserID: userID, MovieID: newMovieID}).Return(
					persistence.Movie{ID: newMovieID, UserID: userID, Status: persistence.MovieStatusPending}, nil,
				)
				gomock.InOrder(
					store.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).DoAndReturn(
						func(_ context.Context, arg persistence.UpdateSessionArg) (persistence.Session, error) {
							require.NotNil(t, arg.WatchedMovieID)
							require.Equal(t, newMovieID, *arg.WatchedMovieID)
							return persistence.Session{ID: sessionID}, nil
						},
					),
					store.EXPECT().UpdateMovies(gomock.Any(), gomock.Any()).DoAndReturn(
						func(_ context.Context, arg persistence.UpdateMoviesArg) error {
							require.Equal(t, newMovieID, arg.ID)
							require.NotNil(t, arg.Status)
							require.Equal(t, persistence.MovieStatusWatched, *arg.Status)
							return nil
						},
					),
				)
			},
		},
		{
			name: "success replacing previously selected movie",
			req:  newReq(&pb.SetSessionMovieRequest{SessionId: sessionID, MovieId: newMovieID}),
			setupMock: func(store *persistencemock.MockTransactionalStorage) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).Return(
					persistence.Session{ID: sessionID, ClosedAt: time.Now(), WinnerID: userID, WatchedMovieID: oldMovieID}, nil,
				)
				store.EXPECT().GetMovie(gomock.Any(), persistence.GetMovieArg{UserID: userID, MovieID: newMovieID}).Return(
					persistence.Movie{ID: newMovieID, UserID: userID, Status: persistence.MovieStatusPending}, nil,
				)
				gomock.InOrder(
					store.EXPECT().UpdateMovies(gomock.Any(), gomock.Any()).DoAndReturn(
						func(_ context.Context, arg persistence.UpdateMoviesArg) error {
							require.Equal(t, oldMovieID, arg.ID)
							require.NotNil(t, arg.Status)
							require.Equal(t, persistence.MovieStatusPending, *arg.Status)
							return nil
						},
					),
					store.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).DoAndReturn(
						func(_ context.Context, arg persistence.UpdateSessionArg) (persistence.Session, error) {
							require.NotNil(t, arg.WatchedMovieID)
							require.Equal(t, newMovieID, *arg.WatchedMovieID)
							return persistence.Session{ID: sessionID}, nil
						},
					),
					store.EXPECT().UpdateMovies(gomock.Any(), gomock.Any()).DoAndReturn(
						func(_ context.Context, arg persistence.UpdateMoviesArg) error {
							require.Equal(t, newMovieID, arg.ID)
							require.NotNil(t, arg.Status)
							require.Equal(t, persistence.MovieStatusWatched, *arg.Status)
							return nil
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
			h := New(store, nil, WithLogger(testLogger()))

			expectTx(store)
			tc.setupMock(store)

			resp, err := h.SetSessionMovie(t.Context(), tc.req)
			if tc.wantErr == nil {
				require.NoError(t, err)
				require.NotNil(t, resp)
				return
			}

			require.Error(t, err)
			require.ErrorAs(t, err, &tc.wantErr)
		})
	}
}

func TestHandlerGetSessionProbabilities(t *testing.T) {
	const sessionID = "session-1"

	type testCase struct {
		name              string
		req               *pb.GetSessionProbabilitiesRequest
		setupMock         func(store *persistencemock.MockTransactionalStorage, lot *mocks.MockLottery)
		wantCode          bool // true => any error
		wantErr           error
		wantProbabilities []*pb.GetSessionProbabilitiesResponse_ParticipantProbabilities
	}

	probability := func(id string, p float64) *pb.GetSessionProbabilitiesResponse_ParticipantProbabilities {
		return &pb.GetSessionProbabilitiesResponse_ParticipantProbabilities{UserId: id, Probability: p}
	}

	testCases := []testCase{
		{
			name: "session not found",
			req:  &pb.GetSessionProbabilitiesRequest{SessionId: sessionID},
			setupMock: func(store *persistencemock.MockTransactionalStorage, _ *mocks.MockLottery) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).
					Return(persistence.Session{}, persistence.ErrNotFound)
			},
			wantErr: persistence.ErrNotFound,
		},
		{
			name: "get session internal error",
			req:  &pb.GetSessionProbabilitiesRequest{SessionId: sessionID},
			setupMock: func(store *persistencemock.MockTransactionalStorage, _ *mocks.MockLottery) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).
					Return(persistence.Session{}, errors.New("boom"))
			},
			wantCode: true,
		},
		{
			name: "list participants error",
			req:  &pb.GetSessionProbabilitiesRequest{SessionId: sessionID},
			setupMock: func(store *persistencemock.MockTransactionalStorage, _ *mocks.MockLottery) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).
					Return(persistence.Session{ID: sessionID}, nil)
				store.EXPECT().ListParticipants(gomock.Any(), persistence.ListParticipantsArg{SessionID: sessionID}).
					Return(persistence.ListParticipantsRet{}, errors.New("boom"))
			},
			wantCode: true,
		},
		{
			name: "no participants returns empty probabilities",
			req:  &pb.GetSessionProbabilitiesRequest{SessionId: sessionID},
			setupMock: func(store *persistencemock.MockTransactionalStorage, _ *mocks.MockLottery) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).
					Return(persistence.Session{ID: sessionID}, nil)
				store.EXPECT().ListParticipants(gomock.Any(), persistence.ListParticipantsArg{SessionID: sessionID}).
					Return(persistence.ListParticipantsRet{}, nil)
			},
			wantProbabilities: nil,
		},
		{
			name: "single participant is certain and skips lottery",
			req:  &pb.GetSessionProbabilitiesRequest{SessionId: sessionID},
			setupMock: func(store *persistencemock.MockTransactionalStorage, _ *mocks.MockLottery) {
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
			setupMock: func(store *persistencemock.MockTransactionalStorage, lot *mocks.MockLottery) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).
					Return(persistence.Session{ID: sessionID}, nil)
				store.EXPECT().ListParticipants(gomock.Any(), persistence.ListParticipantsArg{SessionID: sessionID}).
					Return(persistence.ListParticipantsRet{Participants: []persistence.User{{ID: "u1"}, {ID: "u2"}, {ID: "u3"}}}, nil)
				lot.EXPECT().GetProbabilities(gomock.Any(), lottery.GetProbabilitiesArg{UserIDs: []string{"u1", "u2", "u3"}}).
					Return(lottery.GetProbabilitiesRet{Probabilities: []float64{0.2, 0.3, 0.5}}, nil)
			},
			wantProbabilities: []*pb.GetSessionProbabilitiesResponse_ParticipantProbabilities{
				probability("u1", 0.2), probability("u2", 0.3), probability("u3", 0.5),
			},
		},
		{
			name: "lottery error",
			req:  &pb.GetSessionProbabilitiesRequest{SessionId: sessionID},
			setupMock: func(store *persistencemock.MockTransactionalStorage, lot *mocks.MockLottery) {
				store.EXPECT().GetSession(gomock.Any(), persistence.GetSessionArg{ID: sessionID}).
					Return(persistence.Session{ID: sessionID}, nil)
				store.EXPECT().ListParticipants(gomock.Any(), persistence.ListParticipantsArg{SessionID: sessionID}).
					Return(persistence.ListParticipantsRet{Participants: []persistence.User{{ID: "u1"}, {ID: "u2"}}}, nil)
				lot.EXPECT().GetProbabilities(gomock.Any(), gomock.Any()).
					Return(lottery.GetProbabilitiesRet{}, errors.New("boom"))
			},
			wantCode: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := persistencemock.NewMockTransactionalStorage(ctrl)
			lot := mocks.NewMockLottery(ctrl)
			h := New(store, lot, WithLogger(testLogger()))

			tc.setupMock(store, lot)

			resp, err := h.GetSessionProbabilities(t.Context(), tc.req)
			switch {
			case tc.wantCode:
				require.Error(t, err)
			case tc.wantErr != nil:
				require.Error(t, err)
				require.ErrorIs(t, err, tc.wantErr)
			default:
				require.NoError(t, err)
				require.Len(t, resp.ParticipantsProbabilities, len(tc.wantProbabilities))
				for i, want := range tc.wantProbabilities {
					require.Equal(t, want.UserId, resp.ParticipantsProbabilities[i].UserId)
					require.InDelta(t, want.Probability, resp.ParticipantsProbabilities[i].Probability, 1e-9)
				}
			}
		})
	}
}
