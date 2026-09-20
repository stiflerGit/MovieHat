package requesteditor

import (
	"context"
	"fmt"
	"net/http"
)

func SetBearerToken(token string) func(ctx context.Context, req *http.Request) error {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		return nil
	}
}
