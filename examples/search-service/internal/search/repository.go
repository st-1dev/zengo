package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"zengo/platform/sdk/config"
	"zengo/platform/sdk/errx"
	"zengo/platform/sdk/observability"

	goredis "github.com/redis/go-redis/v9"
	appcfg "github.com/zengo/search-service/gen/api/config/app"
	userhub "github.com/zengo/user-service/gen/api/hub/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	redisclient "zengo/platform/sdk/transport/db/redis"
)

const (
	defaultUserServiceGRPCAddr = "127.0.0.1:9090"
	defaultCacheTTL            = 5 * time.Minute
	allUsersCacheKey           = "search-service:users:all"
)

// Record is the cached search projection of a user.
type Record struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// Repository owns cache lookups and upstream user-service calls.
type Repository struct {
	cache           *goredis.Client
	userServiceAddr string
	cacheTTL        time.Duration
}

// NewRepository builds the search repository used by the generated bootstrap code.
func NewRepository(_ any) *Repository {
	loader := config.NewLoader("configs")
	repo := &Repository{
		userServiceAddr: defaultUserServiceGRPCAddr,
		cacheTTL:        defaultCacheTTL,
	}

	app := loadAppConfig(loader)
	if app != nil && app.GetSpec() != nil {
		addr := strings.TrimSpace(app.GetSpec().GetUserServiceGrpcAddr())
		if addr != "" {
			repo.userServiceAddr = addr
		}
		if app.GetSpec().GetCacheTtl() != nil {
			ttl := app.GetSpec().GetCacheTtl().AsDuration()
			if ttl > 0 {
				repo.cacheTTL = ttl
			}
		}
	}

	redisCfg, err := loader.Redis("redis")
	if err == nil {
		cache, cacheErr := redisclient.New(context.Background(), redisCfg)
		if cacheErr == nil {
			repo.cache = cache
		}
	}
	return repo
}

// GetUser returns one user from cache or user-service.
func (r *Repository) GetUser(ctx context.Context, id string) (*Record, error) {
	if r.cache != nil {
		record, err := r.loadCachedUser(ctx, id)
		if err == nil && record != nil {
			return record, nil
		}
	}

	record, err := r.fetchUser(ctx, id)
	if err != nil {
		return nil, err
	}
	r.cacheUser(ctx, record)
	return record, nil
}

// SearchUsers returns cached users filtered by query and warms the cache from user-service when empty.
func (r *Repository) SearchUsers(ctx context.Context, query string) ([]*Record, error) {
	users, err := r.loadCachedUsers(ctx)
	if err != nil || len(users) == 0 {
		users, err = r.fetchAllUsers(ctx)
		if err != nil {
			return nil, err
		}
		r.cacheUsers(ctx, users)
	}
	return filterUsers(users, query), nil
}

func (r *Repository) loadCachedUser(ctx context.Context, id string) (*Record, error) {
	payload, err := r.cache.Get(ctx, cacheUserKey(id)).Bytes()
	if err != nil {
		return nil, err
	}
	record := &Record{}
	err = json.Unmarshal(payload, record)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *Repository) loadCachedUsers(ctx context.Context) ([]*Record, error) {
	if r.cache == nil {
		return nil, fmt.Errorf("redis cache is unavailable")
	}
	payload, err := r.cache.Get(ctx, allUsersCacheKey).Bytes()
	if err != nil {
		return nil, err
	}
	var users []*Record
	err = json.Unmarshal(payload, &users)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *Repository) fetchUser(ctx context.Context, id string) (*Record, error) {
	client, closeConn, err := r.userClient(ctx)
	if err != nil {
		return nil, err
	}
	defer closeConn()
	var resp *userhub.GetUserResponse

	resp, err = client.GetUser(ctx, &userhub.GetUserRequest{Id: id})
	if err != nil {
		code := errx.Code(err)
		if code == codes.OK {
			code = codes.Internal
		}
		publicMessage := "could not fetch user"
		if code == codes.NotFound {
			publicMessage = "user not found"
		}
		return nil, errx.Wrap(
			err,
			code,
			"fetch user from user-service",
			errx.Public(publicMessage),
			errx.Fields(errx.Field{Key: "user_id", Value: id}),
		)
	}
	if resp.GetUser() == nil {
		return nil, errx.New(
			codes.NotFound,
			"user-service returned empty user",
			errx.Public("user not found"),
			errx.Fields(errx.Field{Key: "user_id", Value: id}),
		)
	}
	return fromUpstreamUser(resp.GetUser()), nil
}

func (r *Repository) fetchAllUsers(ctx context.Context) ([]*Record, error) {
	client, closeConn, err := r.userClient(ctx)
	if err != nil {
		return nil, err
	}
	defer closeConn()
	var resp *userhub.ListUsersResponse

	resp, err = client.ListUsers(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, errx.Wrap(
			err,
			codes.Internal,
			"list users from user-service",
			errx.Public("could not search users"),
		)
	}

	users := make([]*Record, 0, len(resp.GetUsers()))
	for _, user := range resp.GetUsers() {
		users = append(users, fromUpstreamUser(user))
	}
	return users, nil
}

func (r *Repository) userClient(ctx context.Context) (userhub.UserServiceClient, func(), error) {
	_ = ctx
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(errx.UnaryClientInterceptor()),
	}
	dialOpts = append(dialOpts, observability.GRPCDialOptions()...)

	conn, err := grpc.NewClient(r.userServiceAddr, dialOpts...)
	if err != nil {
		return nil, nil, errx.Wrap(
			err,
			codes.Internal,
			"dial user-service",
			errx.Public("could not search users"),
			errx.Fields(errx.Field{Key: "addr", Value: r.userServiceAddr}),
		)
	}
	closeConn := func() {
		_ = conn.Close()
	}
	return userhub.NewUserServiceClient(conn), closeConn, nil
}

func (r *Repository) cacheUser(ctx context.Context, user *Record) {
	if r.cache == nil || user == nil {
		return
	}
	payload, err := json.Marshal(user)
	if err != nil {
		return
	}
	_ = r.cache.Set(ctx, cacheUserKey(user.ID), payload, r.cacheTTL).Err()
}

func (r *Repository) cacheUsers(ctx context.Context, users []*Record) {
	if r.cache == nil || len(users) == 0 {
		return
	}
	payload, err := json.Marshal(users)
	if err != nil {
		return
	}
	_ = r.cache.Set(ctx, allUsersCacheKey, payload, r.cacheTTL).Err()
	for _, user := range users {
		r.cacheUser(ctx, user)
	}
}

func loadAppConfig(loader *config.Loader) *appcfg.Config {
	cfg := &appcfg.Config{}
	err := loader.Get("app", cfg)
	if err != nil {
		return nil
	}
	return cfg
}

func filterUsers(users []*Record, query string) []*Record {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return users
	}
	filtered := make([]*Record, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		if strings.Contains(strings.ToLower(user.ID), query) ||
			strings.Contains(strings.ToLower(user.Email), query) ||
			strings.Contains(strings.ToLower(user.Name), query) ||
			strings.Contains(strings.ToLower(user.DisplayName), query) {
			filtered = append(filtered, user)
		}
	}
	return filtered
}

func fromUpstreamUser(user *userhub.User) *Record {
	if user == nil {
		return nil
	}
	return &Record{
		ID:          user.GetId(),
		Email:       user.GetEmail(),
		Name:        user.GetName(),
		DisplayName: user.GetDisplayName(),
	}
}

func cacheUserKey(id string) string {
	return "search-service:user:" + id
}
