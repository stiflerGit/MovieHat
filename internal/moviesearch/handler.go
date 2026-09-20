package moviesearch

import (
	"context"
	"fmt"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultPerPage = 100
)

type Handler struct {
	provider Provider
}

func New(provider Provider) *Handler {
	return &Handler{provider: provider}
}

func (h *Handler) Search(ctx context.Context, req *pb.SearchMovieRequest) (*pb.SearchMovieResponse, error) {
	resp, err := h.provider.Search(ctx,
		SearchArg{
			Query:   req.Query,
			Page:    int(req.Page),
			PerPage: defaultPerPage,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("h.provider.Search: %w", err)
	}

	searchResponse := &pb.SearchMovieResponse{
		Page:         int32(resp.Page),
		TotalPages:   int32(resp.TotalPages),
		TotalResults: int32(resp.TotalResults),
	}

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

	return searchResponse, nil
}

func (h *Handler) GetByID(ctx context.Context, id string) (*pb.Movie, error) {
	getDetailsRet, err := h.provider.GetDetails(ctx, GetDetailsArg{ID: id})
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
