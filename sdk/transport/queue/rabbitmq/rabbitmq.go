package rabbitmq

import (
	"context"
	"crypto/tls"
	"fmt"
	"zengo/platform/sdk/tlsconfig"

	rabbitmqcfg "zengo/platform/api/config/queue/rabbitmq"
)

// Options describes the validated RabbitMQ connection settings available to service wiring.
type Options struct {
	// URLs lists the AMQP connection URLs.
	URLs []string
	// TLSConfig configures TLS and optional mTLS for AMQP connections.
	TLSConfig *tls.Config
}

// Connect validates typed RabbitMQ config for service-specific AMQP wiring.
//
// The platform does not own a RabbitMQ client abstraction yet, so this helper
// returns validated connection settings for service-specific client code.
func Connect(ctx context.Context, cfg *rabbitmqcfg.Config) (*Options, error) {
	if cfg == nil {
		return nil, fmt.Errorf("rabbitmq config is nil")
	}
	_ = ctx
	if cfg.GetSpec() == nil {
		return nil, fmt.Errorf("rabbitmq spec is nil")
	}
	urls := cfg.GetSpec().GetUrls()
	if len(urls) == 0 {
		return nil, fmt.Errorf("rabbitmq urls are required")
	}
	clientTLS, err := tlsconfig.ClientConfig(tlsconfig.ClientOptionsFromProto(cfg.GetSpec().GetTls()))
	if err != nil {
		return nil, fmt.Errorf("rabbitmq tls: %w", err)
	}
	return &Options{
		URLs:      append([]string(nil), urls...),
		TLSConfig: clientTLS,
	}, nil
}
