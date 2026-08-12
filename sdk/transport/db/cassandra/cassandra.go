package cassandra

import (
	"context"
	"fmt"
	"zengo/platform/sdk/observability"
	"zengo/platform/sdk/tlsconfig"

	cassandracfg "zengo/platform/api/config/db/cassandra"

	"github.com/gocql/gocql"
)

// Connect opens a Cassandra session from a typed config proto.
func Connect(ctx context.Context, cfg *cassandracfg.Config) (*gocql.Session, error) {
	if cfg == nil || cfg.GetSpec() == nil {
		return nil, fmt.Errorf("cassandra config is nil")
	}
	spec := cfg.GetSpec()
	keyspace := spec.GetKeyspace()
	if keyspace == "" {
		keyspace = "default"
	}
	spanCtx, endSpan := observability.StartSpan(
		ctx,
		observability.StringAttribute("db.system", "cassandra"),
		observability.StringAttribute("db.name", keyspace),
	)
	ctx = spanCtx
	defer endSpan()

	hosts := spec.GetHosts()
	if len(hosts) == 0 {
		hosts = []string{"localhost:9042"}
	}
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = spec.GetKeyspace()
	cluster.Consistency = gocql.Quorum
	clientTLS, err := tlsconfig.ClientConfig(tlsconfig.ClientOptionsFromProto(spec.GetTls()))
	if err != nil {
		observability.RecordException(ctx, err, "cassandra tls")
		return nil, fmt.Errorf("cassandra tls: %w", err)
	}
	if clientTLS != nil {
		cluster.SslOpts = &gocql.SslOptions{
			Config:                 clientTLS,
			EnableHostVerification: !clientTLS.InsecureSkipVerify,
		}
	}
	if spec.Username != nil && spec.GetUsername() != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: spec.GetUsername(),
			Password: spec.GetPassword(),
		}
	}
	var session *gocql.Session
	session, err = cluster.CreateSession()
	if err != nil {
		observability.RecordException(ctx, err, "connect cassandra")
		return nil, fmt.Errorf("connect cassandra: %w", err)
	}
	return session, nil
}
