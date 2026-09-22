package pagination

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

type testFilters struct {
	UserID string
	Status int
}

func TestToken_Validate(t *testing.T) {
	tests := []struct {
		name    string
		token   Token[string]
		want    string
		wantErr error
	}{
		{name: "match", token: Token[string]{Value: "alien"}, want: "alien"},
		{name: "mismatch", token: Token[string]{Value: "alien"}, want: "aliens", wantErr: ErrTokenMismatch},
		{name: "zero token vs query", token: Token[string]{}, want: "alien", wantErr: ErrTokenMismatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.token.Validate(tt.want)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEncode_Decode_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		want Token[string]
	}{
		{name: "populated token", want: Token[string]{Value: "alien", NextOffset: 42}},
		{name: "zero token", want: Token[string]{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := EncodeToken(tt.want)
			require.NoError(t, err)

			got, err := DecodeToken[string](raw)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestEncode_Decode_RoundTrip_StructPayload(t *testing.T) {
	want := Token[testFilters]{Value: testFilters{UserID: "u-1", Status: 2}, NextOffset: 10}

	raw, err := EncodeToken(want)
	require.NoError(t, err)

	got, err := DecodeToken[testFilters](raw)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Token[string]
		wantErr bool
	}{
		{
			name:  "empty input yields zero token",
			input: "",
			want:  Token[string]{},
		},
		{
			name:    "malformed base64",
			input:   "!!!",
			wantErr: true,
		},
		{
			name:    "valid base64, invalid json",
			input:   base64.RawURLEncoding.EncodeToString([]byte("not json")),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeToken[string](tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
