package redis

import (
	"context"
	"fmt"
	"math"
	"zengo/platform/sdk/observability"
	"zengo/platform/sdk/tlsconfig"

	rediscfg "zengo/platform/api/config/db/redis"

	"github.com/redis/go-redis/extra/redisotel/v9"
	goredis "github.com/redis/go-redis/v9"
)

// ClientFromPort builds a minimal redis client for local development.
func ClientFromPort(ctx context.Context, host string, port, db int) (*goredis.Client, error) {
	if port <= 0 || port > math.MaxUint16 {
		return nil, fmt.Errorf("redis port must be between 1 and 65535")
	}
	if db < 0 || db > math.MaxInt32 {
		return nil, fmt.Errorf("redis db overflows int32: %d", db)
	}
	if host == "" {
		host = "localhost"
	}
	db32 := int32(db)
	return New(ctx, &rediscfg.Config{
		Kind: "redis",
		Spec: &rediscfg.Spec{
			Addrs: []string{fmt.Sprintf("%s:%d", host, port)},
			Db:    &db32,
		},
	})
}

// New opens and validates a traced go-redis client from typed config.
//
// The caller owns the returned client and must close it on shutdown.
func New(ctx context.Context, cfg *rediscfg.Config) (*goredis.Client, error) {
	if cfg == nil || cfg.GetSpec() == nil {
		return nil, fmt.Errorf("redis config is nil")
	}
	spec := cfg.GetSpec()
	dbName := fmt.Sprintf("db%d", spec.GetDb())
	spanCtx, endSpan := observability.StartSpan(
		ctx,
		observability.StringAttribute("db.system", "redis"),
		observability.StringAttribute("db.name", dbName),
	)
	ctx = spanCtx
	defer endSpan()

	opts, err := options(spec)
	if err != nil {
		observability.RecordException(ctx, err, "build redis options")
		return nil, err
	}
	client := goredis.NewClient(opts)
	err = redisotel.InstrumentTracing(client)
	if err != nil {
		_ = client.Close()
		observability.RecordException(ctx, err, "instrument redis tracing")
		return nil, fmt.Errorf("instrument redis tracing: %w", err)
	}
	err = client.Ping(ctx).Err()
	if err != nil {
		_ = client.Close()
		observability.RecordException(ctx, err, "connect redis")
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	return client, nil
}

func options(spec *rediscfg.Spec) (*goredis.Options, error) {
	if spec == nil {
		return nil, fmt.Errorf("redis spec is nil")
	}
	url := spec.GetUrl()
	if url != "" {
		opts, err := goredis.ParseURL(url)
		if err != nil {
			return nil, fmt.Errorf("parse redis url: %w", err)
		}
		clientTLS, tlsErr := tlsconfig.ClientConfig(tlsconfig.ClientOptionsFromProto(spec.GetTls()))
		if tlsErr != nil {
			return nil, fmt.Errorf("redis tls: %w", tlsErr)
		}
		if clientTLS != nil {
			opts.TLSConfig = clientTLS
		}
		applySpecOptions(opts, spec)
		return opts, nil
	}
	addrs := spec.GetAddrs()
	if len(addrs) == 0 {
		addrs = []string{"localhost:6379"}
	}
	opts := &goredis.Options{
		Addr:     addrs[0],
		Username: spec.GetUsername(),
	}
	if spec.Password != nil {
		opts.Password = spec.GetPassword()
	}
	if spec.Db != nil {
		opts.DB = int(spec.GetDb())
	}
	clientTLS, err := tlsconfig.ClientConfig(tlsconfig.ClientOptionsFromProto(spec.GetTls()))
	if err != nil {
		return nil, fmt.Errorf("redis tls: %w", err)
	}
	if clientTLS != nil {
		opts.TLSConfig = clientTLS
	}
	applySpecOptions(opts, spec)
	return opts, nil
}

func applySpecOptions(opts *goredis.Options, spec *rediscfg.Spec) {
	if spec.PoolSize != nil && spec.GetPoolSize() > 0 {
		opts.PoolSize = int(spec.GetPoolSize())
	}
	if spec.DialTimeout != nil {
		opts.DialTimeout = spec.GetDialTimeout().AsDuration()
	}
	if spec.ReadTimeout != nil {
		opts.ReadTimeout = spec.GetReadTimeout().AsDuration()
	}
}
