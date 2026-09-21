package tmdb

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stiflerGit/moviehat/internal/moviesearch"
	"github.com/stiflerGit/moviehat/internal/moviesearch/provider/tmdb/mocks"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func httpResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestNew(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mocks.NewMockClientInterface(ctrl)
	p := New(client)
	require.NotNil(t, p)
}

func TestProvider_Search(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mocks.NewMockClientInterface(ctrl)
	p := New(client)

	client.EXPECT().
		SearchMulti(gomock.Any(), gomock.Any()).
		Return(httpResp(200, `{"page":1,"total_pages":1,"total_results":1,"results":[{"id":550,"title":"Fight Club","release_date":"1999-10-15"}]}`), nil)

	ret, err := p.Search(t.Context(), moviesearch.SearchArg{Query: "Fight Club", Page: 1})
	require.NoError(t, err)
	require.Len(t, ret.Results, 1)
	require.Equal(t, "550", ret.Results[0].ID)
	require.Equal(t, "Fight Club", ret.Results[0].Title)
}

func TestProvider_Search_HTTPError(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mocks.NewMockClientInterface(ctrl)
	p := New(client)

	client.EXPECT().SearchMulti(gomock.Any(), gomock.Any()).Return(nil, io.ErrUnexpectedEOF)

	_, err := p.Search(t.Context(), moviesearch.SearchArg{Query: "xyz"})
	require.Error(t, err)
}

func TestProvider_Search_NilJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mocks.NewMockClientInterface(ctrl)
	p := New(client)

	client.EXPECT().SearchMulti(gomock.Any(), gomock.Any()).Return(httpResp(200, `{}`), nil)

	_, err := p.Search(t.Context(), moviesearch.SearchArg{Query: "test"})
	require.Error(t, err)
}

func TestProvider_GetDetails_Movie(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mocks.NewMockClientInterface(ctrl)
	p := New(client)

	client.EXPECT().
		MovieDetails(gomock.Any(), int32(550), gomock.Any()).
		Return(httpResp(200, `{"id":550,"title":"Fight Club","release_date":"1999-10-15"}`), nil)

	ret, err := p.GetDetails(t.Context(), moviesearch.GetDetailsArg{ID: "550"})
	require.NoError(t, err)
	require.Equal(t, "550", ret.Result.ID)
	require.Equal(t, "Fight Club", ret.Result.Title)
}

func TestProvider_GetDetails_MovieNotFound_FallsBackToTV(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mocks.NewMockClientInterface(ctrl)
	p := New(client)

	client.EXPECT().
		MovieDetails(gomock.Any(), int32(1396), gomock.Any()).
		Return(httpResp(404, `Not found`), nil)
	client.EXPECT().
		TvSeriesDetails(gomock.Any(), int32(1396), gomock.Any()).
		Return(httpResp(200, `{"id":1396,"name":"Breaking Bad","first_air_date":"2008-01-20"}`), nil)

	ret, err := p.GetDetails(t.Context(), moviesearch.GetDetailsArg{ID: "1396"})
	require.NoError(t, err)
	require.Equal(t, "1396", ret.Result.ID)
	require.Equal(t, "Breaking Bad", ret.Result.Title)
}

func TestProvider_GetDetails_BothNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mocks.NewMockClientInterface(ctrl)
	p := New(client)

	client.EXPECT().
		MovieDetails(gomock.Any(), int32(999), gomock.Any()).
		Return(httpResp(404, `Not found`), nil)
	client.EXPECT().
		TvSeriesDetails(gomock.Any(), int32(999), gomock.Any()).
		Return(httpResp(404, `Not found`), nil)

	_, err := p.GetDetails(t.Context(), moviesearch.GetDetailsArg{ID: "999"})
	require.Error(t, err)
	require.ErrorIs(t, err, moviesearch.NotFoundErr)
}

func TestProvider_GetDetails_InvalidID(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mocks.NewMockClientInterface(ctrl)
	p := New(client)

	_, err := p.GetDetails(t.Context(), moviesearch.GetDetailsArg{ID: "not-a-number"})
	require.Error(t, err)
}

func TestProvider_GetDetails_MovieDetails_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mocks.NewMockClientInterface(ctrl)
	p := New(client)

	client.EXPECT().
		MovieDetails(gomock.Any(), int32(1), gomock.Any()).
		Return(httpResp(200, `{}`), nil)

	_, err := p.GetDetails(t.Context(), moviesearch.GetDetailsArg{ID: "1"})
	require.Error(t, err)
}
