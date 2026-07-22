package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateCryptoSecureRandomString(t *testing.T) {
	a, err := generateCryptoSecureRandomString()
	require.NoError(t, err)
	require.Len(t, a, 43)

	b, err := generateCryptoSecureRandomString()
	require.NoError(t, err)
	require.Len(t, b, 43)
	require.NotEqual(t, a, b)
}

func TestTokenHash(t *testing.T) {
	hash, err := tokenHash("token", "secret")
	require.NoError(t, err)
	require.Equal(t, "6UERDj0r/oJiHw4+FDRzDXMF0QbF9oyHFl0LJ6RhGko", hash)
}
