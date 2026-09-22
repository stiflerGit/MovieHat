package moviesearch

import (
	"context"
	"errors"
	"fmt"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/pkg/pagination"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Handler struct {
	searcher           MoviesSearcher
	movieDetailsGetter MovieDetailsGetter
}

func New(
	provider MoviesSearcher,
	movieDetailsGetter MovieDetailsGetter,
) *Handler {
	return &Handler{
		searcher:           provider,
		movieDetailsGetter: movieDetailsGetter,
	}
}

func (h *Handler) Search(ctx context.Context, req *pb.SearchMovieRequest) (*pb.SearchMovieResponse, error) {
	var pageToken pagination.Token[string]
	var err error

	pageToken, err = pagination.DecodeToken[string](req.PageToken)
	if err != nil {
		return nil, fmt.Errorf("pagination.DecodeToken: %w", err)
	}

	if err = pageToken.Validate(req.Query); err != nil {
		if errors.Is(err, pagination.ErrTokenMismatch) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid page_token"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp, err := h.searcher.SearchMovies(ctx,
		SearchMoviesArg{
			Query:  req.Query,
			Offset: pageToken.NextOffset,
			Limit:  int(req.PageSize),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("h.provider.Search: %w", err)
	}

	searchResponse := &pb.SearchMovieResponse{}
	searchResponse.Results = make([]*pb.SearchMovieResponse_Result, 0, len(resp.Results))
	for _, r := range resp.Results {
		searchResponse.Results = append(searchResponse.Results,
			&pb.SearchMovieResponse_Result{
				Id:          r.ID,
				Title:       r.Title,
				ReleaseDate: timestamppb.New(r.ReleaseDate),
				PosterPath:  r.PosterPath,
			},
		)
	}

	if resp.HasMore {
		v, err := pagination.EncodeToken(pagination.Token[string]{Value: req.Query, NextOffset: pageToken.NextOffset + int(req.PageSize)})
		if err != nil {
			return nil, fmt.Errorf("encodePageToken: %w", err)
		}
		searchResponse.NextPageToken = string(v)
	}

	return searchResponse, nil
}

func (h *Handler) GetByID(ctx context.Context, id string) (*pb.Movie, error) {
	getDetailsRet, err := h.movieDetailsGetter.GetDetails(ctx, GetDetailsArg{ID: id})
	if err != nil {
		return nil, fmt.Errorf("h.provider.GetDetails(%q): %w", id, err)
	}

	return &pb.Movie{
		Id:          id,
		Title:       getDetailsRet.Result.Title,
		ReleaseDate: timestamppb.New(getDetailsRet.Result.ReleaseDate),
		PosterPath:  getDetailsRet.Result.PosterPath,
	}, nil
}
