package list

import (
	"testing"
	"time"

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

func TestRepoSessionToPBSession(t *testing.T) {
	closedAt := time.Now().Round(time.Second)
	s := repoSessionToPBSession(persistence.Session{ID: "s1", WinnerID: "u1", CreatedAt: time.Unix(1, 0), ClosedAt: closedAt})

	require.Equal(t, "s1", s.Id)
	require.Equal(t, "u1", s.Winner.Id)
	require.True(t, s.ClosedAt.IsValid())
	require.True(t, s.ClosedAt.AsTime().Equal(closedAt))
}

func TestRepoSessionsToPB(t *testing.T) {
	sessions := repoSessionsToPB(persistence.Session{ID: "s1", CreatedAt: time.Unix(1, 0)})
	require.Len(t, sessions, 1)
	require.Equal(t, "s1", sessions[0].Id)

	require.Nil(t, repoSessionsToPB())
}

func TestRepoMoviesToPB(t *testing.T) {
	movies := repoMoviesToPB(persistence.Movie{Title: "Alien"}, persistence.Movie{Title: "Dune"})
	require.Len(t, movies, 2)
	require.Equal(t, "Alien", movies[0].Title)
	require.Equal(t, "Dune", movies[1].Title)
}
