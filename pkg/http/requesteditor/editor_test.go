package requesteditor

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetBearerToken(t *testing.T) {
	const token = "my-secret-token"
	fn := SetBearerToken(token)
	require.NotNil(t, fn)

	req, err := http.NewRequestWithContext(t.Context(), "GET", "/", nil)
	require.NoError(t, err)

	err = fn(t.Context(), req)
	require.NoError(t, err)
	require.Equal(t, "Bearer my-secret-token", req.Header.Get("Authorization"))
}
