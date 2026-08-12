package versioning

import (
	"strings"
)

// HubMeta describes the canonical hub API layout discovered from generated Go packages.
type HubMeta struct {
	// GoImport is the Go import path for the hub package root.
	GoImport string
	// RelativePath is the path below gen/api/hub that contains the primary package.
	RelativePath string
	// PackageSuffix is the protobuf package suffix, typically "hub".
	PackageSuffix string
	// PrimaryService is the first non-event hub service discovered in the schema.
	PrimaryService string
	// EventService is the first hub event handler service discovered in the schema.
	EventService string
}

// DiscoverHubMeta discovers HubMeta for the generated hub API packages.
func DiscoverHubMeta(module, genDir string) (HubMeta, error) {
	if activeLoader != nil {
		return activeLoader.HubMeta()
	}
	l, err := NewLoader(module, genDir, nil)
	if err != nil {
		return HubMeta{}, err
	}
	return l.HubMeta()
}

func hubImportPath(module string, meta HubMeta) string {
	if meta.GoImport != "" && strings.HasPrefix(meta.GoImport, module) {
		return meta.GoImport
	}
	return module + "/gen/api/hub/" + meta.RelativePath
}

func legacyImportPath(module, version string, meta HubMeta) string {
	return module + "/gen/api/" + version + "/" + meta.RelativePath
}

func serviceBaseName(service string) string {
	return strings.TrimSuffix(service, "Service")
}

func registerGRPCName(service, version string) string {
	uc := strings.ToUpper(version[:1]) + version[1:]
	return "RegisterGRPC" + serviceBaseName(service) + uc
}

func gatewayRegisterName(service, version string) string {
	uc := strings.ToUpper(version[:1]) + version[1:]
	return "GatewayRegister" + serviceBaseName(service) + uc
}

func legacyWireGRPCName(service, version string) string {
	uc := strings.ToUpper(version[:1]) + version[1:]
	return "RegisterLegacy" + service + uc
}

func legacyWireEventName(service, version string) string {
	uc := strings.ToUpper(version[:1]) + version[1:]
	return "RegisterLegacy" + service + uc
}

func adapterTypeName(service, version string) string {
	uc := strings.ToUpper(version[:1]) + version[1:]
	return service + uc
}

func adapterFileName(service string) string {
	return strings.ToLower(serviceBaseName(service)) + "_service_gen.go"
}

func eventAdapterFileName(service string) string {
	base := strings.TrimSuffix(service, "EventHandler")
	return strings.ToLower(base) + "_event_handler_gen.go"
}
