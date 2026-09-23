// Package requesteditor provides request editors for the generated TMDB client.
package requesteditor

import (
	"context"
	"fmt"
	"net/http"
)

// SetBearerToken returns a request editor that authenticates outgoing TMDB
// requests with a Bearer token.
func SetBearerToken(token string) func(ctx context.Context, req *http.Request) error {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		return nil
	}
}
