package versioning

import (
	"os"
	"path/filepath"
	"strings"
	"zengo/platform/internal/codegen"
)

type runtimeTemplateData struct {
	HubImport         string
	Service           string
	HubRegisterFunc   string
	HubGatewayFunc    string
	LegacyVersions    []runtimeLegacyVersionData
	HasEventService   bool
	KafkaHubRegistrar string
}

type runtimeLegacyVersionData struct {
	Version       string
	RegisterFunc  string
	GatewayFunc   string
	AdapterAlias  string
	AdapterImport string
	TypeName      string
}

func generateRuntime(layout Layout, opts GenerateOptions) error {
	meta := opts.HubMeta
	service := meta.PrimaryService
	dir := filepath.Join(opts.GenDir, "zengo")
	data := runtimeTemplateData{
		HubImport:       hubImportPath(opts.Module, meta),
		Service:         service,
		HubRegisterFunc: registerGRPCName(service, "hub"),
		HubGatewayFunc:  gatewayRegisterName(service, "hub"),
		HasEventService: meta.EventService != "",
	}
	for _, v := range layout.Legacy {
		data.LegacyVersions = append(data.LegacyVersions, runtimeLegacyVersionData{
			Version:       v,
			RegisterFunc:  legacyWireGRPCName(service, v),
			GatewayFunc:   gatewayRegisterName(service, v),
			AdapterAlias:  "adapter" + v,
			AdapterImport: opts.Module + "/gen/zengo/adapters/" + v,
			TypeName:      adapterTypeName(service, v),
		})
	}
	if data.HasEventService {
		data.KafkaHubRegistrar = kafkaHubRegistrarName(opts)
	}
	return codegen.Render(codegen.File{
		Template:   "runtime.go.tmpl",
		Data:       data,
		OutputPath: filepath.Join(dir, "runtime_gen.go"),
	})
}

func kafkaHubRegistrarName(opts GenerateOptions) string {
	path := filepath.Join(opts.GenDir, "zengo", "register_hub.pb.go")
	data, err := os.ReadFile(path)
	if err != nil {
		return "Register" + opts.HubMeta.EventService + "Hub"
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "func Register") && strings.Contains(line, "Hub(consumer *kafka.Consumer") {
			name := strings.TrimPrefix(line, "func ")
			idx := strings.Index(name, "(")
			if idx > 0 {
				return name[:idx]
			}
		}
	}
	return "Register" + opts.HubMeta.EventService + "Hub"
}
