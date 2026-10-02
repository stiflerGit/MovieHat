package user

import (
	"testing"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/persistence"

	"github.com/stretchr/testify/require"
)

func TestPBUserToRepoUser(t *testing.T) {
	u := pbUserToRepoUser(&pb.User{Id: "u1", Name: "john"})
	require.Equal(t, "u1", u.ID)
	require.Equal(t, "john", u.Name)
}

func TestRepoUserToPBUser(t *testing.T) {
	u := repoUserToPBUser(persistence.User{ID: "u1", Name: "john"})
	require.Equal(t, "u1", u.Id)
	require.Equal(t, "john", u.Name)
}
