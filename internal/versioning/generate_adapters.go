package versioning

import (
	"path/filepath"
	"strings"
	"zengo/platform/internal/codegen"
)

type adapterTemplateData struct {
	Version       string
	Service       string
	TypeName      string
	HubImport     string
	LegacyImport  string
	ConvertImport string
	NeedsStatus   bool
	RPCs          []adapterRPCData
}

type adapterRPCData struct {
	Name                       string
	RequestType                string
	ResponseType               string
	Mode                       string
	Reason                     string
	IsEmptyRequest             bool
	HasRepeatedMessageResponse bool
	ListField                  string
	ElemType                   string
}

func generateAdapters(version string, plan ConversionPlan, legacy *Schema, opts GenerateOptions) error {
	meta := opts.HubMeta
	service := meta.PrimaryService
	dir := filepath.Join(opts.GenDir, "zengo", "adapters", version)
	rpcModes := map[string]string{}
	for _, rpc := range plan.RPCs {
		rpcModes[rpc.LegacyRPC.Name] = rpc.Mode
	}

	data := adapterTemplateData{
		Version:       version,
		Service:       service,
		TypeName:      adapterTypeName(service, version),
		HubImport:     hubImportPath(opts.Module, meta),
		LegacyImport:  legacyImportPath(opts.Module, version, meta),
		ConvertImport: opts.Module + "/gen/zengo/convert/" + version,
	}

	for _, svc := range legacy.Services {
		if svc.Name == meta.EventService || strings.HasSuffix(svc.Name, "EventHandler") {
			continue
		}
		for _, rpc := range svc.RPCs {
			rpcData := adapterRPCData{
				Name:           rpc.Name,
				RequestType:    rpc.RequestType,
				ResponseType:   rpc.ResponseType,
				Mode:           rpcModes[rpc.Name],
				IsEmptyRequest: rpc.RequestType == "Empty",
			}
			if rpcData.Mode == "UNIMPLEMENTED" {
				rpcData.Reason = rpc.Name + " removed from hub"
				data.NeedsStatus = true
			}
			listField, elemType, ok := repeatedMessageField(legacy, rpc.ResponseType)
			if ok && rpc.RequestType == "Empty" {
				rpcData.HasRepeatedMessageResponse = true
				rpcData.ListField = listField
				rpcData.ElemType = elemType
			}
			data.RPCs = append(data.RPCs, rpcData)
		}
	}

	return codegen.Render(codegen.File{
		Template:   "adapter_service.go.tmpl",
		Data:       data,
		OutputPath: filepath.Join(dir, adapterFileName(service)),
	})
}

func repeatedMessageField(schema *Schema, messageName string) (fieldName, elemType string, ok bool) {
	msg, exists := schema.Messages[messageName]
	if !exists {
		return "", "", false
	}
	for _, f := range msg.Fields {
		if !f.Repeated {
			continue
		}
		if codegen.IsScalarType(f.Type) {
			continue
		}
		return f.Name, f.Type, true
	}
	return "", "", false
}
