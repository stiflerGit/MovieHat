package tests

import (
	"fmt"
	"net/http"
	"testing"

	gatewayv1 "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/api/gateway/v1/gatewayv1connect"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullSession(t *testing.T) {
	// TODO: add server init
	ctx := t.Context()
	client := gatewayv1connect.NewGatewayServiceClient(http.DefaultClient, "http://localhost:8080")
	// sign in with admin
	signInResponse, err := client.SignIn(ctx, &gatewayv1.SignInRequest{Email: "admin@moviehat.com", Password: "moviehat"})
	require.NoError(t, err)
	ctx, callInfo := connect.NewClientContext(ctx)
	callInfo.RequestHeader().Add("Authorization", fmt.Sprintf("Bearer %s", signInResponse.Token))
	// create an invitation
	createInvitationResp, err := client.CreateInvitation(ctx, &gatewayv1.CreateInvitationRequest{})
	require.NoError(t, err)
	// use the invitation to create 2 users
	// user 1
	signUpResp, err := client.SignUp(ctx, &gatewayv1.SignUpRequest{Email: "u1@moviehat.com", Password: "moviehat1", InvitationToken: createInvitationResp.InvitationToken})
	require.NoError(t, err)
	user1Token := signUpResp.Token
	ctx, callInfo = connect.NewClientContext(ctx)
	callInfo.RequestHeader().Add("Authorization", fmt.Sprintf("Bearer %s", user1Token))
	updateUser1Resp, err := client.UpdateUser(ctx, &gatewayv1.UpdateUserRequest{Name: "user1"})
	require.NoError(t, err)
	// user 2
	signUpResp, err = client.SignUp(ctx, &gatewayv1.SignUpRequest{Email: "u2@moviehat.com", Password: "moviehat2", InvitationToken: createInvitationResp.InvitationToken})
	require.NoError(t, err)
	user2Token := signUpResp.Token
	ctx, callInfo = connect.NewClientContext(ctx)
	callInfo.RequestHeader().Add("Authorization", fmt.Sprintf("Bearer %s", user2Token))
	updateUser2Resp, err := client.UpdateUser(ctx, &gatewayv1.UpdateUserRequest{Name: "user2"})
	require.NoError(t, err)

	// add films for each users
	ctx, callInfo = connect.NewClientContext(ctx)
	callInfo.RequestHeader().Add("Authorization", fmt.Sprintf("Bearer %s", user1Token))
	user1Movie1, err := client.AddUserMovie(ctx, &gatewayv1.AddUserMovieRequest{MovieTitle: "movie11", Note: "note11"})
	require.NoError(t, err)
	_, err = client.AddUserMovie(ctx, &gatewayv1.AddUserMovieRequest{MovieTitle: "movie12", Note: "note12"})
	require.NoError(t, err)
	_, err = client.AddUserMovie(ctx, &gatewayv1.AddUserMovieRequest{MovieTitle: "movie13", Note: "note13"})
	require.NoError(t, err)

	ctx, callInfo = connect.NewClientContext(ctx)
	callInfo.RequestHeader().Add("Authorization", fmt.Sprintf("Bearer %s", user2Token))
	user2Movie1, err := client.AddUserMovie(ctx, &gatewayv1.AddUserMovieRequest{MovieTitle: "movie21", Note: "note11"})
	require.NoError(t, err)
	_, err = client.AddUserMovie(ctx, &gatewayv1.AddUserMovieRequest{MovieTitle: "movie22", Note: "note12"})
	require.NoError(t, err)
	_, err = client.AddUserMovie(ctx, &gatewayv1.AddUserMovieRequest{MovieTitle: "movie23", Note: "note13"})
	require.NoError(t, err)
	// begin a session
	createSessionResp, err := client.CreateSession(ctx, &gatewayv1.CreateSessionRequest{})
	require.NoError(t, err)
	// add participants to a session
	client.AddParticipant(ctx, &gatewayv1.AddParticipantRequest{SessionId: createSessionResp.Session.Id, UserId: updateUser1Resp.User.Id})
	require.NoError(t, err)
	client.AddParticipant(ctx, &gatewayv1.AddParticipantRequest{SessionId: createSessionResp.Session.Id, UserId: updateUser2Resp.User.Id})
	require.NoError(t, err)
	// check probabilities
	getSessionProbabilities, err := client.GetSessionProbabilities(ctx, &gatewayv1.GetSessionProbabilitiesRequest{SessionId: createSessionResp.Session.Id})
	require.NoError(t, err)
	assert.ElementsMatch(t, []*gatewayv1.GetSessionProbabilitiesResponse_ParticipantProbabilities{
		{
			UserId:      updateUser1Resp.User.Id,
			Probability: 0.5,
		},
		{
			UserId:      updateUser2Resp.User.Id,
			Probability: 0.5,
		},
	}, getSessionProbabilities.ParticipantsProbabilities)
	// end the session
	endSessionResp, err := client.EndSession(ctx, &gatewayv1.EndSessionRequest{Id: createSessionResp.Session.Id})
	require.NoError(t, err)
	// winner select a film

	setSessionMovieReq := &gatewayv1.SetSessionMovieRequest{SessionId: createSessionResp.Session.Id, MovieId: user1Movie1.Movie.Id}
	ctx, callInfo = connect.NewClientContext(ctx)
	callInfo.RequestHeader().Add("Authorization", fmt.Sprintf("Bearer %s", user1Token))
	if endSessionResp.Winner.Id == updateUser2Resp.User.Id {
		ctx, callInfo = connect.NewClientContext(ctx)
		callInfo.RequestHeader().Add("Authorization", fmt.Sprintf("Bearer %s", user2Token))
		setSessionMovieReq = &gatewayv1.SetSessionMovieRequest{SessionId: createSessionResp.Session.Id, MovieId: user2Movie1.Movie.Id}
	}

	_, err = client.SetSessionMovie(ctx, setSessionMovieReq)
	require.NoError(t, err)

	// TODO: when status is available in movie add an assertion to check that status of the selected movie is watched
	getSessionResp, err := client.GetSession(ctx, &gatewayv1.GetSessionRequest{Id: createSessionResp.Session.Id})
	require.NoError(t, err)

	assert.Equal(t, createSessionResp.Session.Id, getSessionResp.Session.Id)
	assert.NotNil(t, getSessionResp.Session.ClosedAt)
	assert.ElementsMatch(t, []*gatewayv1.User{
		{
			Id:   updateUser1Resp.User.Id,
			Name: updateUser1Resp.User.Name,
		},
		{
			Id:   updateUser2Resp.User.Id,
			Name: updateUser2Resp.User.Name,
		},
	}, getSessionResp.Session.Participants)

	assert.Equal(t, endSessionResp.Winner.Id, getSessionResp.Session.Winner.Id)
	// TODO: add watched movie id
}
