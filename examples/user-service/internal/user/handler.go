package user

import (
	"context"
	"encoding/json"
	"errors"
	"zengo/platform/sdk/config"
	"zengo/platform/sdk/errx"
	"zengo/platform/sdk/observability"
	"zengo/platform/sdk/router"
	"zengo/platform/sdk/transport/queue/kafka"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"

	appcfg "github.com/zengo/user-service/gen/api/config/app"
	userhub "github.com/zengo/user-service/gen/api/hub/user"
	"github.com/zengo/user-service/gen/zengo"
)

// Handler implements the example user service and its event consumer.
type Handler struct {
	userhub.UnimplementedUserServiceServer
	userhub.UnimplementedUserEventHandlerServer
	repo     *Repository
	producer *kafka.Producer
	brokers  []string
	app      *appcfg.Config
}

// NewHandler builds the example service handler and loads optional app config when omitted.
func NewHandler(repo *Repository, producer *kafka.Producer, brokers []string, app ...*appcfg.Config) *Handler {
	if len(brokers) == 0 {
		brokers = []string{"localhost:9092"}
	}
	cfg := firstAppConfig(app)
	if cfg == nil {
		cfg = loadAppConfig()
	}
	return &Handler{repo: repo, producer: producer, brokers: brokers, app: cfg}
}

// RegisterKafka connects the generated Kafka registration helpers to the example event handler.
func RegisterKafka(consumer *kafka.Consumer, deps zengo.Dependencies, hubHandler userhub.UserEventHandlerServer) error {
	_ = hubHandler
	return zengo.RegisterKafka(consumer, deps, func(ctx context.Context, env router.EventEnvelope) error {
		return HandleUserCreatedEvent(ctx, env.Payload)
	})
}

// GetUser loads one user and maps storage errors to transport-safe application errors.
func (h *Handler) GetUser(ctx context.Context, req *userhub.GetUserRequest) (*userhub.GetUserResponse, error) {
	ctx, endFunc := observability.StartSpan(ctx, observability.StringAttribute("aa", "vv"))
	defer endFunc()

	user, err := h.repo.GetByID(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errx.Wrap(
				err,
				codes.NotFound,
				"get user by id",
				errx.Public("user not found"),
			)
		}
		return nil, errx.Wrap(
			err,
			codes.Internal,
			"get user by id",
			errx.Public("could not fetch user"),
		)
	}
	return &userhub.GetUserResponse{User: toProto(user)}, nil
}

// ListUsers returns all users known to the example service.
func (h *Handler) ListUsers(ctx context.Context, _ *emptypb.Empty) (*userhub.ListUsersResponse, error) {
	users, err := h.repo.List(ctx)
	if err != nil {
		return nil, errx.Wrap(
			err,
			codes.Internal,
			"list users",
			errx.Public("could not list users"),
		)
	}
	out := make([]*userhub.User, 0, len(users))
	for _, u := range users {
		out = append(out, toProto(u))
	}
	return &userhub.ListUsersResponse{Users: out}, nil
}

// CreateUser validates input, persists the user, and publishes a user-created event.
func (h *Handler) CreateUser(ctx context.Context, req *userhub.CreateUserRequest) (*userhub.CreateUserResponse, error) {
	if req.GetEmail() == "" || req.GetName() == "" {
		return nil, errx.New(
			codes.InvalidArgument,
			"email and name are required",
			errx.Public("email and name are required"),
		)
	}
	displayName := req.GetDisplayName()
	if h.requiresDisplayName() && displayName == "" {
		return nil, errx.New(
			codes.InvalidArgument,
			"display_name is required",
			errx.Public("display_name is required"),
		)
	}
	if displayName == "" {
		displayName = h.defaultDisplayName(req.GetName())
	}

	user, err := h.repo.Create(ctx, req.GetEmail(), displayName)
	if err != nil {
		return nil, errx.Wrap(
			err,
			codes.Internal,
			"create user",
			errx.Public("could not create user"),
		)
	}

	pbUser := toProto(user)
	event := &userhub.UserCreatedEvent{ApiVersion: "hub", User: pbUser}
	var payload []byte

	payload, err = protojson.Marshal(event)
	if err != nil {
		return nil, errx.Wrap(
			err,
			codes.Internal,
			"marshal user created event",
			errx.Public("could not create user"),
		)
	}

	err = zengo.PublishCreateUser(ctx, h.producer, h.brokers, "hub", payload)
	if err != nil {
		return nil, errx.Wrap(
			err,
			codes.Internal,
			"publish user created event",
			errx.Public("could not create user"),
		)
	}
	return &userhub.CreateUserResponse{User: pbUser}, nil
}

// OnUserCreated handles the hub event endpoint for user-created events.
func (h *Handler) OnUserCreated(ctx context.Context, event *userhub.UserCreatedEvent) (*emptypb.Empty, error) {
	payload, err := protojson.Marshal(event)
	if err != nil {
		return nil, errx.Wrap(
			err,
			codes.Internal,
			"marshal user created event",
			errx.Public("could not process user event"),
		)
	}

	err = HandleUserCreatedEvent(ctx, payload)
	if err != nil {
		return nil, errx.Wrap(
			err,
			codes.Internal,
			"handle user created event",
			errx.Public("could not process user event"),
		)
	}
	return &emptypb.Empty{}, nil
}

func toProto(user *Record) *userhub.User {
	return &userhub.User{
		Id:          user.ID,
		Email:       user.Email,
		Name:        user.Name,
		DisplayName: user.Name,
	}
}

// HandleUserCreatedEvent decodes a user-created event from protobuf or JSON payload bytes.
func HandleUserCreatedEvent(ctx context.Context, payload []byte) error {
	var event userhub.UserCreatedEvent
	err := protojson.Unmarshal(payload, &event)
	if err != nil {
		err = json.Unmarshal(payload, &event)
		if err != nil {
			return err
		}
	}
	_ = ctx
	_ = event.GetApiVersion()
	return nil
}

func (h *Handler) defaultDisplayName(name string) string {
	prefix := ""
	if h != nil && h.app != nil && h.app.GetSpec() != nil {
		prefix = h.app.GetSpec().GetDefaultDisplayNamePrefix()
	}
	if prefix == "" {
		return name
	}
	return prefix + name
}

func (h *Handler) requiresDisplayName() bool {
	return h != nil &&
		h.app != nil &&
		h.app.GetSpec() != nil &&
		h.app.GetSpec().GetRequireDisplayName()
}

func firstAppConfig(app []*appcfg.Config) *appcfg.Config {
	if len(app) == 0 {
		return nil
	}
	return app[0]
}

func loadAppConfig() *appcfg.Config {
	loader := config.NewLoader("configs")
	cfg := &appcfg.Config{}
	err := loader.Get("app", cfg)
	if err != nil {
		return nil
	}
	return cfg
}
