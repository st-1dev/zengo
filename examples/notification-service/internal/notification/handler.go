package notification

import (
	"context"
	"fmt"
	"zengo/platform/sdk/router"
	"zengo/platform/sdk/transport/queue/kafka"

	notificationhub "github.com/zengo/notification-service/gen/api/hub/notification"
	"github.com/zengo/notification-service/gen/zengo"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Handler prints incoming user-created notifications to stdout.
type Handler struct {
	notificationhub.UnimplementedNotificationServiceServer
	notificationhub.UnimplementedNotificationEventHandlerServer
}

// NewHandler builds the notification consumer handler.
func NewHandler(repo *Repository, producer *kafka.Producer, brokers []string) *Handler {
	_ = repo
	_ = producer
	_ = brokers
	return &Handler{}
}

// GetStatus reports that the notification consumer is ready.
func (h *Handler) GetStatus(ctx context.Context, _ *emptypb.Empty) (*notificationhub.GetStatusResponse, error) {
	_ = h
	_ = ctx
	return &notificationhub.GetStatusResponse{
		Service: "notification-service",
		State:   "ready",
	}, nil
}

// OnUserCreated prints the received user-created event.
func (h *Handler) OnUserCreated(ctx context.Context, event *notificationhub.UserCreatedEvent) (*emptypb.Empty, error) {
	_ = h
	_ = ctx
	printUserCreated(event)
	return &emptypb.Empty{}, nil
}

// RegisterKafka connects generated Kafka routing to the handler logic.
func RegisterKafka(
	consumer *kafka.Consumer,
	deps zengo.Dependencies,
	hubHandler notificationhub.NotificationEventHandlerServer,
) error {
	return zengo.RegisterKafka(consumer, deps, func(ctx context.Context, env router.EventEnvelope) error {
		return handleUserCreatedEvent(ctx, hubHandler, env.Payload)
	})
}

func handleUserCreatedEvent(
	ctx context.Context,
	hubHandler notificationhub.NotificationEventHandlerServer,
	payload []byte,
) error {
	event := &notificationhub.UserCreatedEvent{}
	err := protojson.Unmarshal(payload, event)
	if err != nil {
		return fmt.Errorf("decode user created event: %w", err)
	}
	_, err = hubHandler.OnUserCreated(ctx, event)
	if err != nil {
		return err
	}
	return nil
}

func printUserCreated(event *notificationhub.UserCreatedEvent) {
	if event == nil || event.GetUser() == nil {
		fmt.Println("notification-service: received empty user.created event")
		return
	}
	user := event.GetUser()
	fmt.Printf(
		"notification-service: user created id=%s email=%s name=%s display_name=%s api_version=%s\n",
		user.GetId(),
		user.GetEmail(),
		user.GetName(),
		user.GetDisplayName(),
		event.GetApiVersion(),
	)
}
