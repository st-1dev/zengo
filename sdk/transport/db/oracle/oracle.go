package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"zengo/platform/sdk/observability"
	"zengo/platform/sdk/tlsconfig"

	oraclecfg "zengo/platform/api/config/db/oracle"

	go_ora "github.com/sijms/go-ora/v3"
)

const defaultPort = 1521

// DSN builds an Oracle connection string for go-ora v3.
func DSN(cfg *oraclecfg.Config) string {
	if cfg == nil || cfg.GetSpec() == nil {
		return ""
	}
	spec := cfg.GetSpec()
	port := defaultPort
	if spec.Port != nil && spec.GetPort() > 0 {
		port = int(spec.GetPort())
	}
	password := ""
	if spec.Password != nil {
		password = spec.GetPassword()
	}
	return go_ora.BuildUrl(spec.GetHost(), port, spec.GetServiceName(), spec.GetUserName(), password, nil)
}

// Connect opens and validates a database/sql Oracle client from a typed config proto.
//
// The caller owns the returned database handle and must close it on shutdown.
func Connect(ctx context.Context, cfg *oraclecfg.Config) (*sql.DB, error) {
	err := validateConfig(cfg)
	if err != nil {
		return nil, err
	}
	spec := cfg.GetSpec()
	spanCtx, endSpan := observability.StartSpan(
		ctx,
		observability.StringAttribute("db.system", "oracle"),
		observability.StringAttribute("db.name", spec.GetServiceName()),
	)
	ctx = spanCtx
	defer endSpan()

	connector := go_ora.NewConnector(DSN(cfg))
	clientTLS, tlsErr := tlsconfig.ClientConfig(tlsconfig.ClientOptionsFromProto(spec.GetTls()))
	if tlsErr != nil {
		observability.RecordException(ctx, tlsErr, "oracle tls")
		return nil, fmt.Errorf("oracle tls: %w", tlsErr)
	}
	if clientTLS != nil {
		oracleConnector, ok := connector.(*go_ora.OracleConnector)
		if !ok {
			err = fmt.Errorf("unexpected oracle connector type %T", connector)
			observability.RecordException(ctx, err, "oracle connector type assertion")
			return nil, err
		}
		oracleConnector.WithTLSConfig(clientTLS)
	}
	db := sql.OpenDB(connector)
	err = db.PingContext(ctx)
	if err != nil {
		_ = db.Close()
		observability.RecordException(ctx, err, "connect oracle")
		return nil, fmt.Errorf("connect oracle: %w", err)
	}
	return db, nil
}

func validateConfig(cfg *oraclecfg.Config) error {
	if cfg == nil || cfg.GetSpec() == nil {
		return fmt.Errorf("oracle config is nil")
	}
	spec := cfg.GetSpec()
	if spec.GetHost() == "" {
		return fmt.Errorf("oracle host is required")
	}
	if spec.GetServiceName() == "" {
		return fmt.Errorf("oracle service_name is required")
	}
	if spec.GetUserName() == "" {
		return fmt.Errorf("oracle user_name is required")
	}
	return nil
}
