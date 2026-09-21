package tmdb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_mapTMDBSearchMultiResponseToDomain(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		want     int
		wantID   string
		wantName string
	}{
		{
			name: "movie result",
			json: `{
				"page": 1, "total_pages": 1, "total_results": 1,
				"results": [{"id": 550, "title": "Fight Club", "release_date": "1999-10-15", "poster_path": "/poster.jpg"}]
			}`,
			want:     1,
			wantID:   "550",
			wantName: "Fight Club",
		},
		{
			name: "tv result uses name when title absent",
			json: `{
				"page": 1, "total_pages": 1, "total_results": 1,
				"results": [{"id": 1396, "name": "Breaking Bad", "first_air_date": "2008-01-20", "poster_path": "/bb.jpg"}]
			}`,
			want:     1,
			wantID:   "1396",
			wantName: "Breaking Bad",
		},
		{
			name: "empty results",
			json: `{
				"page": 1, "total_pages": 1, "total_results": 0,
				"results": []
			}`,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := searchMultiResponse(tt.json)
			got := mapTMDBSearchMultiResponseToDomain(r)
			require.Equal(t, tt.want, len(got.Results))

			if len(got.Results) > 0 {
				require.Equal(t, tt.wantID, got.Results[0].ID)
				require.Equal(t, tt.wantName, got.Results[0].Title)
			}
		})
	}
}

func Test_mapTMDBTvSeriesDetailsResponseToDomain(t *testing.T) {
	// map assumes validate was called first; only test valid input.
	tests := []struct {
		name   string
		json   string
		wantID string
	}{
		{name: "valid tv series", json: `{"id":1396,"name":"Breaking Bad","first_air_date":"2008-01-20","poster_path":"/bb.jpg"}`, wantID: "1396"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tvSeriesDetailsResponse(tt.json)
			require.NoError(t, validateTVSeriesDetailsResponse(r))
			got, err := mapTMDBTvSeriesDetailsResponseToDomain(r)
			require.NoError(t, err)
			require.Equal(t, tt.wantID, got.Result.ID)
			require.Equal(t, "Breaking Bad", got.Result.Title)
		})
	}
}

func Test_mapTMDBMovieDetailsResponseToDomain(t *testing.T) {
	// map assumes validate was called first; only test valid input.
	tests := []struct {
		name   string
		json   string
		wantID string
	}{
		{name: "valid movie", json: `{"id":550,"title":"Fight Club","release_date":"1999-10-15","poster_path":"/poster.jpg"}`, wantID: "550"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := movieDetailsResponse(tt.json)
			require.NoError(t, validateMovieDetailsResponse(r))
			got, err := mapTMDBMovieDetailsResponseToDomain(r)
			require.NoError(t, err)
			require.Equal(t, tt.wantID, got.Result.ID)
			require.Equal(t, "Fight Club", got.Result.Title)
		})
	}
}
