package tmdb

import (
	"encoding/json"
	"testing"

	tmdb "github.com/stiflerGit/moviehat/gen/tmdb"

	"github.com/stretchr/testify/require"
)

func searchMovieResponse(jsonBody string) *tmdb.SearchMovieResponse {
	r := &tmdb.SearchMovieResponse{}
	if jsonBody != "" {
		_ = json.Unmarshal([]byte(`{"JSON200": `+jsonBody+`}`), r)
	}
	return r
}

func Test_validateSearchMovieResponse(t *testing.T) {
	tests := []struct {
		name string
		r    *tmdb.SearchMovieResponse
		err  bool
	}{
		{name: "nil response", r: nil, err: true},
		{name: "nil json200", r: &tmdb.SearchMovieResponse{}, err: true},
		{name: "nil page", r: searchMovieResponse(`{"total_pages":1,"total_results":1}`), err: true},
		{name: "nil total_pages", r: searchMovieResponse(`{"page":1,"total_results":1}`), err: true},
		{name: "nil total_results", r: searchMovieResponse(`{"page":1,"total_pages":1}`), err: true},
		{name: "valid", r: searchMovieResponse(`{"page":1,"total_pages":10,"total_results":100}`), err: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSearchMovieResponse(tt.r)
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
