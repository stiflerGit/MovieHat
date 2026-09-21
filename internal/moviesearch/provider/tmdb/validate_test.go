package tmdb

import (
	"encoding/json"
	"testing"

	tmdb "github.com/stiflerGit/moviehat/gen/tmdb"

	"github.com/stretchr/testify/require"
)

func searchMultiResponse(jsonBody string) *tmdb.SearchMultiResponse {
	r := &tmdb.SearchMultiResponse{}
	if jsonBody != "" {
		_ = json.Unmarshal([]byte(`{"JSON200": `+jsonBody+`}`), r)
	}
	return r
}

func Test_validateSearchMultiResponse(t *testing.T) {
	tests := []struct {
		name string
		r    *tmdb.SearchMultiResponse
		err  bool
	}{
		{name: "nil response", r: nil, err: true},
		{name: "nil json200", r: &tmdb.SearchMultiResponse{}, err: true},
		{name: "nil page", r: searchMultiResponse(`{"total_pages":1,"total_results":1}`), err: true},
		{name: "nil total_pages", r: searchMultiResponse(`{"page":1,"total_results":1}`), err: true},
		{name: "nil total_results", r: searchMultiResponse(`{"page":1,"total_pages":1}`), err: true},
		{name: "valid", r: searchMultiResponse(`{"page":1,"total_pages":10,"total_results":100}`), err: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSearchMultiResponse(tt.r)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func movieDetailsResponse(jsonBody string) *tmdb.MovieDetailsResponse {
	r := &tmdb.MovieDetailsResponse{}
	if jsonBody != "" {
		_ = json.Unmarshal([]byte(`{"JSON200": `+jsonBody+`}`), r)
	}
	return r
}

func Test_validateMovieDetailsResponse(t *testing.T) {
	tests := []struct {
		name string
		r    *tmdb.MovieDetailsResponse
		err  bool
	}{
		{name: "nil response", r: nil, err: true},
		{name: "nil json200", r: &tmdb.MovieDetailsResponse{}, err: true},
		{name: "nil id", r: movieDetailsResponse(`{"title":"Alien"}`), err: true},
		{name: "nil title", r: movieDetailsResponse(`{"id":42}`), err: true},
		{name: "valid", r: movieDetailsResponse(`{"id":42,"title":"Alien"}`), err: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMovieDetailsResponse(tt.r)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func tvSeriesDetailsResponse(jsonBody string) *tmdb.TvSeriesDetailsResponse {
	r := &tmdb.TvSeriesDetailsResponse{}
	if jsonBody != "" {
		_ = json.Unmarshal([]byte(`{"JSON200": `+jsonBody+`}`), r)
	}
	return r
}

func Test_validateTVSeriesDetailsResponse(t *testing.T) {
	tests := []struct {
		name string
		r    *tmdb.TvSeriesDetailsResponse
		err  bool
	}{
		{name: "nil response", r: nil, err: true},
		{name: "nil json200", r: &tmdb.TvSeriesDetailsResponse{}, err: true},
		{name: "nil id", r: tvSeriesDetailsResponse(`{"name":"Stranger Things"}`), err: true},
		{name: "nil name", r: tvSeriesDetailsResponse(`{"id":99}`), err: true},
		{name: "valid", r: tvSeriesDetailsResponse(`{"id":99,"name":"Stranger Things"}`), err: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTVSeriesDetailsResponse(tt.r)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
