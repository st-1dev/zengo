package nats

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"zengo/platform/sdk/observability"
	"zengo/platform/sdk/tlsconfig"

	natscfg "zengo/platform/api/config/queue/nats"

	"github.com/nats-io/nats.go"
)

// Connect opens a NATS connection from typed config.
func Connect(ctx context.Context, cfg *natscfg.Config) (*nats.Conn, error) {
	if cfg == nil || cfg.GetSpec() == nil {
		return nil, fmt.Errorf("nats config is nil")
	}
	urls := strings.Join(cfg.GetSpec().GetUrls(), ",")
	spanCtx, endSpan := observability.StartSpan(
		ctx,
		observability.StringAttribute("messaging.system", "nats"),
		observability.StringAttribute("messaging.destination.name", urls),
	)
	ctx = spanCtx
	defer endSpan()

	opts, err := options(cfg.GetSpec())
	if err != nil {
		observability.RecordException(ctx, err, "build nats options")
		return nil, err
	}
	var conn *nats.Conn
	conn, err = nats.Connect(opts.URL, opts.NatsOptions...)
	if err != nil {
		observability.RecordException(ctx, err, "connect nats")
		return nil, fmt.Errorf("connect nats: %w", err)
	}
	return conn, nil
}

type parsedOptions struct {
	URL         string
	NatsOptions []nats.Option
}

func options(spec *natscfg.Spec) (*parsedOptions, error) {
	if spec == nil {
		return nil, fmt.Errorf("nats spec is nil")
	}
	urls := spec.GetUrls()
	if len(urls) == 0 {
		urls = []string{nats.DefaultURL}
	}
	out := &parsedOptions{URL: strings.Join(urls, ",")}
	name := spec.GetName()
	if name != "" {
		out.NatsOptions = append(out.NatsOptions, nats.Name(name))
	}
	token := spec.GetToken()
	if token != "" {
		out.NatsOptions = append(out.NatsOptions, nats.Token(token))
	}
	if spec.Username != nil && spec.GetUsername() != "" {
		out.NatsOptions = append(out.NatsOptions, nats.UserInfo(spec.GetUsername(), spec.GetPassword()))
	}
	if spec.MaxReconnects != nil {
		out.NatsOptions = append(out.NatsOptions, nats.MaxReconnects(int(spec.GetMaxReconnects())))
	}
	if spec.ConnectTimeout != nil {
		out.NatsOptions = append(out.NatsOptions, nats.Timeout(spec.GetConnectTimeout().AsDuration()))
	}
	clientTLS, err := tlsconfig.ClientConfig(tlsconfig.ClientOptionsFromProto(spec.GetTls()))
	if err != nil {
		return nil, fmt.Errorf("nats tls: %w", err)
	}
	if clientTLS != nil {
		out.NatsOptions = append(out.NatsOptions, nats.Secure(clientTLS))
	}
	return out, nil
}

// ConnectDefault connects to nats://127.0.0.1:4222 for local development.
func ConnectDefault(ctx context.Context) (*nats.Conn, error) {
	return Connect(ctx, &natscfg.Config{
		Kind: "nats",
		Spec: &natscfg.Spec{Urls: []string{nats.DefaultURL}},
	})
}

// PortFromEnv parses a development port override and falls back on invalid input.
func PortFromEnv(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port <= 0 || port > math.MaxUint16 {
		return fallback
	}
	return port
}
