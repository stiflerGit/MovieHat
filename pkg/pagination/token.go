// Package pagination provides helpers for offset-based pagination: an Adapter
// for fetching windows from fixed-size sources, and opaque page tokens.
package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrTokenMismatch = errors.New("page token does not match request")

// Token is the decoded content of an opaque page token. Value identifies the
// request the token belongs to (e.g. the search query), so that a token used
// against a different request can be detected and rejected with ErrMismatch.
// NextOffset is the offset to resume from; the zero Token is the first page.
type Token[T comparable] struct {
	Value      T   `json:"value"`
	NextOffset int `json:"next_offset"`
}

// Validate reports whether the token was issued for v, returning ErrMismatch
// otherwise.
func (t *Token[T]) Validate(v T) error {
	if t.NextOffset > 0 && v != t.Value {
		return ErrTokenMismatch
	}

	return nil
}

// EncodeToken serializes t into an opaque, URL-safe page token.
func EncodeToken[T comparable](t Token[T]) (string, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// DecodeToken parses a token produced by EncodeToken. An empty input decodes
// to the zero Token (first page). Token values are opaque: treat them as black
// boxes and always check them with Validate against the current request.
func DecodeToken[T comparable](v string) (Token[T], error) {
	var t Token[T]

	if len(v) == 0 {
		return t, nil
	}

	buf, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return t, fmt.Errorf("base64.RawURLEncoding.DecodeString: %w", err)
	}

	if err = json.Unmarshal(buf, &t); err != nil {
		return t, fmt.Errorf("json.Unmarshal: %w", err)
	}

	return t, nil
}
