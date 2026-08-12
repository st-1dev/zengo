package search

import (
	"context"
	"strings"
	"zengo/platform/sdk/errx"

	searchhub "github.com/zengo/search-service/gen/api/hub/search"
	"google.golang.org/grpc/codes"
)

// Handler serves search requests backed by Redis and the upstream user service.
type Handler struct {
	searchhub.UnimplementedSearchServiceServer
	repo *Repository
}

// NewHandler builds the search API handler.
func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// GetUser returns a cached user or fetches it from user-service on cache miss.
func (h *Handler) GetUser(ctx context.Context, req *searchhub.GetUserRequest) (*searchhub.GetUserResponse, error) {
	if strings.TrimSpace(req.GetId()) == "" {
		return nil, errx.New(
			codes.InvalidArgument,
			"user id is required",
			errx.Public("user id is required"),
		)
	}
	user, err := h.repo.GetUser(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &searchhub.GetUserResponse{User: toProto(user)}, nil
}

// SearchUsers searches cached users and falls back to user-service when the cache is empty.
func (h *Handler) SearchUsers(
	ctx context.Context,
	req *searchhub.SearchUsersRequest,
) (*searchhub.SearchUsersResponse, error) {
	users, err := h.repo.SearchUsers(ctx, req.GetQuery())
	if err != nil {
		return nil, err
	}
	out := make([]*searchhub.User, 0, len(users))
	for _, user := range users {
		out = append(out, toProto(user))
	}
	return &searchhub.SearchUsersResponse{Users: out}, nil
}

func toProto(user *Record) *searchhub.User {
	if user == nil {
		return nil
	}
	return &searchhub.User{
		Id:          user.ID,
		Email:       user.Email,
		Name:        user.Name,
		DisplayName: user.DisplayName,
	}
}
