package generator

import (
	"fmt"
	"zengo/platform/internal/codegen"
	"zengo/platform/internal/manifest"
)

// GenerateMain renders the generated service bootstrap for manifest into outPath.
func GenerateMain(m *manifest.Manifest, outPath string) error {
	if m != nil && m.Transports.REST != nil && m.Transports.GRPC == nil {
		return fmt.Errorf("manifest.transports.grpc is required when manifest.transports.rest is enabled")
	}
	if m != nil && m.Transports.REST != nil && m.Transports.GRPC != nil && m.Transports.GRPC.TLS != nil &&
		m.Transports.REST.GRPCClientTLS == nil {
		return fmt.Errorf(
			"manifest.transports.rest.grpc_client_tls is required when manifest.transports.grpc.tls is enabled",
		)
	}
	return codegen.Render(codegen.File{
		Template:   "main.go.tmpl",
		Data:       m,
		OutputPath: outPath,
	})
}
