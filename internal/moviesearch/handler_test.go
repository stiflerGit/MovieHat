package moviesearch_test

import (
	"testing"
	"time"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/moviesearch"
	"github.com/stiflerGit/moviehat/internal/moviesearch/mocks"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func searchRet(results ...moviesearch.SearchMoviesRetResult) moviesearch.SearchMoviesRet {
	return moviesearch.SearchMoviesRet{Results: results}
}

func TestHandler_Search(t *testing.T) {
	ctrl := gomock.NewController(t)
	searcher := mocks.NewMockMoviesSearcher(ctrl)
	h := moviesearch.New(searcher, mocks.NewMockMovieDetailsGetter(ctrl))

	searcher.EXPECT().SearchMovies(gomock.Any(), moviesearch.SearchMoviesArg{Query: "Fight Club", Offset: 0, Limit: 100}).
		Return(searchRet(moviesearch.SearchMoviesRetResult{
			ID:          "550",
			Title:       "Fight Club",
			ReleaseDate: time.Date(1999, 10, 15, 0, 0, 0, 0, time.UTC),
			PosterPath:  "/poster.jpg",
		}), nil)

	resp, err := h.Search(t.Context(), &pb.SearchMovieRequest{Query: "Fight Club", PageSize: 100})
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	require.Equal(t, "550", resp.Results[0].Id)
	require.Equal(t, "Fight Club", resp.Results[0].Title)
	require.True(t, resp.Results[0].ReleaseDate.AsTime().Equal(time.Date(1999, 10, 15, 0, 0, 0, 0, time.UTC)))
}

func TestHandler_Search_HasMoreReturnsToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	searcher := mocks.NewMockMoviesSearcher(ctrl)
	h := moviesearch.New(searcher, mocks.NewMockMovieDetailsGetter(ctrl))

	searcher.EXPECT().SearchMovies(gomock.Any(), moviesearch.SearchMoviesArg{Query: "Fight Club", Offset: 0, Limit: 100}).
		Return(moviesearch.SearchMoviesRet{Results: []moviesearch.SearchMoviesRetResult{{ID: "550"}}, HasMore: true}, nil)

	resp, err := h.Search(t.Context(), &pb.SearchMovieRequest{Query: "Fight Club", PageSize: 100})
	require.NoError(t, err)
	require.NotEmpty(t, resp.NextPageToken)
}

func TestHandler_Search_HasMoreFalseReturnsEmptyToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	searcher := mocks.NewMockMoviesSearcher(ctrl)
	h := moviesearch.New(searcher, mocks.NewMockMovieDetailsGetter(ctrl))

	searcher.EXPECT().SearchMovies(gomock.Any(), moviesearch.SearchMoviesArg{Query: "Fight Club", Offset: 0, Limit: 100}).
		Return(searchRet(moviesearch.SearchMoviesRetResult{ID: "550"}), nil)

	resp, err := h.Search(t.Context(), &pb.SearchMovieRequest{Query: "Fight Club", PageSize: 100})
	require.NoError(t, err)
	require.Empty(t, resp.NextPageToken)
}

// The token is opaque by contract: this only exercises the round trip through
// the public API, asserting the searcher receives the right offset.
func TestHandler_Search_PageTokenRoundTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	searcher := mocks.NewMockMoviesSearcher(ctrl)
	h := moviesearch.New(searcher, mocks.NewMockMovieDetailsGetter(ctrl))

	searcher.EXPECT().SearchMovies(gomock.Any(), moviesearch.SearchMoviesArg{Query: "Fight Club", Offset: 0, Limit: 100}).
		Return(moviesearch.SearchMoviesRet{Results: []moviesearch.SearchMoviesRetResult{{ID: "550"}}, HasMore: true}, nil)
	first, err := h.Search(t.Context(), &pb.SearchMovieRequest{Query: "Fight Club", PageSize: 100})
	require.NoError(t, err)
	require.NotEmpty(t, first.NextPageToken)

	searcher.EXPECT().SearchMovies(gomock.Any(), moviesearch.SearchMoviesArg{Query: "Fight Club", Offset: 100, Limit: 50}).
		Return(searchRet(), nil)
	second, err := h.Search(t.Context(), &pb.SearchMovieRequest{Query: "Fight Club", PageSize: 50, PageToken: first.NextPageToken})
	require.NoError(t, err)
	require.Empty(t, second.NextPageToken)
}

func TestHandler_Search_InvalidPageToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := moviesearch.New(mocks.NewMockMoviesSearcher(ctrl), mocks.NewMockMovieDetailsGetter(ctrl))

	_, err := h.Search(t.Context(), &pb.SearchMovieRequest{Query: "xyz", PageSize: 10, PageToken: "not a token"})
	require.Error(t, err)
}

func TestHandler_Search_ProviderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	searcher := mocks.NewMockMoviesSearcher(ctrl)
	h := moviesearch.New(searcher, mocks.NewMockMovieDetailsGetter(ctrl))

	searcher.EXPECT().SearchMovies(gomock.Any(), gomock.Any()).Return(moviesearch.SearchMoviesRet{}, errExpected)

	_, err := h.Search(t.Context(), &pb.SearchMovieRequest{Query: "xyz"})
	require.Error(t, err)
}

func TestHandler_GetByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	getter := mocks.NewMockMovieDetailsGetter(ctrl)
	h := moviesearch.New(mocks.NewMockMoviesSearcher(ctrl), getter)

	getter.EXPECT().GetDetails(gomock.Any(), moviesearch.GetDetailsArg{ID: "550"}).
		Return(moviesearch.GetDetailsRet{
			Result: moviesearch.SearchMoviesRetResult{
				ID:          "550",
				Title:       "Fight Club",
				ReleaseDate: time.Date(1999, 10, 15, 0, 0, 0, 0, time.UTC),
				PosterPath:  "/poster.jpg",
			},
		}, nil)

	movie, err := h.GetByID(t.Context(), "550")
	require.NoError(t, err)
	require.Equal(t, "550", movie.Id)
	require.Equal(t, "Fight Club", movie.Title)
	require.True(t, movie.ReleaseDate.AsTime().Equal(time.Date(1999, 10, 15, 0, 0, 0, 0, time.UTC)))
}

func TestHandler_GetByID_ProviderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	getter := mocks.NewMockMovieDetailsGetter(ctrl)
	h := moviesearch.New(mocks.NewMockMoviesSearcher(ctrl), getter)

	getter.EXPECT().GetDetails(gomock.Any(), gomock.Any()).Return(moviesearch.GetDetailsRet{}, errExpected)

	_, err := h.GetByID(t.Context(), "999")
	require.Error(t, err)
}

var errExpected = errSentinel{}

type errSentinel struct{}

func (errSentinel) Error() string { return "expected error" }
