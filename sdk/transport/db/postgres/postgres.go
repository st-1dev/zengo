package postgres

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
	"zengo/platform/sdk/observability"
	"zengo/platform/sdk/tlsconfig"

	postgrescfg "zengo/platform/api/config/db/postgres"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DSN builds a PostgreSQL DSN from a typed config proto.
func DSN(cfg *postgrescfg.Config) string {
	spec := cfg.GetSpec()
	port := int32(5432)
	if spec.Port != nil {
		port = *spec.Port
	}
	password := ""
	if spec.Password != nil {
		password = *spec.Password
	}
	sslMode := "disable"
	if spec.SslMode != nil {
		sslMode = stringsLowerSSLMode(*spec.SslMode)
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(spec.UserName, password),
		Host:   fmt.Sprintf("%s:%d", spec.Host, port),
		Path:   spec.DbName,
	}
	q := u.Query()
	q.Set("sslmode", sslMode)
	if len(spec.Schemas) > 0 {
		q.Set("search_path", joinSchemas(spec.Schemas))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// NewPool opens and returns a traced pgxpool.Pool from typed config.
//
// The caller owns the returned pool and must close it when the service shuts down.
func NewPool(ctx context.Context, cfg *postgrescfg.Config) (*pgxpool.Pool, error) {
	spec := cfg.GetSpec()
	spanCtx, endSpan := observability.StartSpan(
		ctx,
		observability.StringAttribute("db.system", "postgresql"),
		observability.StringAttribute("db.name", spec.GetDbName()),
	)
	ctx = spanCtx
	defer endSpan()

	poolCfg, err := pgxpool.ParseConfig(DSN(cfg))
	if err != nil {
		observability.RecordException(ctx, err, "parse postgres dsn")
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	clientTLS, tlsErr := tlsconfig.ClientConfig(tlsconfig.ClientOptionsFromProto(spec.GetTls()))
	if tlsErr != nil {
		observability.RecordException(ctx, tlsErr, "postgres tls")
		return nil, fmt.Errorf("postgres tls: %w", tlsErr)
	}
	if clientTLS != nil {
		poolCfg.ConnConfig.TLSConfig = clientTLS
	}
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()
	conn := spec.GetConnection()
	if conn != nil {
		if conn.MaxOpen > 0 {
			if conn.MaxOpen > math.MaxInt32 {
				return nil, fmt.Errorf("postgres max_open overflows int32: %d", conn.MaxOpen)
			}
			poolCfg.MaxConns = int32(conn.MaxOpen)
		}
		if conn.MinOpen > 0 {
			if conn.MinOpen > math.MaxInt32 {
				return nil, fmt.Errorf("postgres min_open overflows int32: %d", conn.MinOpen)
			}
			poolCfg.MinConns = int32(conn.MinOpen)
		}
		if conn.MaxIdle > 0 {
			poolCfg.MaxConnIdleTime = time.Duration(conn.MaxIdle) * time.Second
		}
		if conn.MaxLifeTime != nil {
			poolCfg.MaxConnLifetime = conn.MaxLifeTime.AsDuration()
		}
		if conn.MaxIdleTime != nil {
			poolCfg.MaxConnIdleTime = conn.MaxIdleTime.AsDuration()
		}
	}
	var pool *pgxpool.Pool
	pool, err = pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		observability.RecordException(ctx, err, "connect postgres")
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return pool, nil
}

func stringsLowerSSLMode(mode postgrescfg.Spec_SSLMode) string {
	switch mode {
	case postgrescfg.Spec_PREFER:
		return "prefer"
	case postgrescfg.Spec_ALLOW:
		return "allow"
	case postgrescfg.Spec_REQUIRE:
		return "require"
	case postgrescfg.Spec_VERIFY_CA:
		return "verify-ca"
	case postgrescfg.Spec_VERIFY_FULL:
		return "verify-full"
	default:
		return "disable"
	}
}

func joinSchemas(schemas []string) string {
	var out strings.Builder
	for i, s := range schemas {
		if i > 0 {
			out.WriteString(",")
		}
		out.WriteString(s)
	}
	return out.String()
}

// PoolConfigFromPort builds a minimal postgres config for local development.
func PoolConfigFromPort(host string, port int, dbName, user, password string) (*postgrescfg.Config, error) {
	if port <= 0 || port > math.MaxUint16 {
		return nil, fmt.Errorf("postgres port must be between 1 and 65535")
	}
	port32 := int32(port)
	return &postgrescfg.Config{
		Kind:       "postgres",
		ApiVersion: "v1",
		Spec: &postgrescfg.Spec{
			Host:     host,
			Port:     &port32,
			DbName:   dbName,
			UserName: user,
			Password: &password,
		},
	}, nil
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
