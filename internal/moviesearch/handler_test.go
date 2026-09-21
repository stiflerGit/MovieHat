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

func TestHandler_Search(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := mocks.NewMockProvider(ctrl)
	h := moviesearch.New(provider)

	provider.EXPECT().Search(gomock.Any(), moviesearch.SearchArg{Query: "Fight Club", Page: 1, PerPage: 100}).
		Return(moviesearch.SearchRet{
			Page:         1,
			TotalPages:   1,
			TotalResults: 1,
			Results: []moviesearch.SearchRetResult{{
				ID:          "550",
				Title:       "Fight Club",
				ReleaseDate: time.Date(1999, 10, 15, 0, 0, 0, 0, time.UTC),
				PosterPath:  "/poster.jpg",
			}},
		}, nil)

	resp, err := h.Search(t.Context(), &pb.SearchMovieRequest{Query: "Fight Club", Page: 1})
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Page)
	require.Len(t, resp.Results, 1)
	require.Equal(t, "550", resp.Results[0].Id)
	require.Equal(t, "Fight Club", resp.Results[0].Title)
}

func TestHandler_Search_ProviderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := mocks.NewMockProvider(ctrl)
	h := moviesearch.New(provider)

	provider.EXPECT().Search(gomock.Any(), gomock.Any()).Return(moviesearch.SearchRet{}, errExpected)

	_, err := h.Search(t.Context(), &pb.SearchMovieRequest{Query: "xyz"})
	require.Error(t, err)
}

func TestHandler_GetByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := mocks.NewMockProvider(ctrl)
	h := moviesearch.New(provider)

	provider.EXPECT().GetDetails(gomock.Any(), moviesearch.GetDetailsArg{ID: "550"}).
		Return(moviesearch.GetDetailsRet{
			Result: moviesearch.SearchRetResult{
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
	provider := mocks.NewMockProvider(ctrl)
	h := moviesearch.New(provider)

	provider.EXPECT().GetDetails(gomock.Any(), gomock.Any()).Return(moviesearch.GetDetailsRet{}, errExpected)

	_, err := h.GetByID(t.Context(), "999")
	require.Error(t, err)
}

var errExpected = errSentinel{}

type errSentinel struct{}

func (errSentinel) Error() string { return "expected error" }
