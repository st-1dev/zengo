package versioning

import (
	"path/filepath"
	"zengo/platform/internal/codegen"
)

type eventAdapterTemplateData struct {
	Version       string
	VersionUC     string
	EventService  string
	HubImport     string
	LegacyImport  string
	ConvertImport string
	RegistrarName string
	RPCs          []eventRPCData
}

type eventRPCData struct {
	Name        string
	RequestType string
	Consume     *eventConsumeData
}

type eventConsumeData struct {
	Topic string
	Group string
}

func generateEventAdapters(version string, legacy *Schema, opts GenerateOptions) error {
	meta := opts.HubMeta
	if meta.EventService == "" {
		return nil
	}
	var eventSvc *Service
	for i := range legacy.Services {
		if legacy.Services[i].Name == meta.EventService {
			eventSvc = &legacy.Services[i]
			break
		}
	}
	if eventSvc == nil {
		return nil
	}

	dir := filepath.Join(opts.GenDir, "zengo", "adapters", version)
	legacyImport := legacyImportPath(opts.Module, version, meta)
	data := eventAdapterTemplateData{
		Version:       version,
		VersionUC:     codegen.UcFirst(version),
		EventService:  meta.EventService,
		HubImport:     hubImportPath(opts.Module, meta),
		LegacyImport:  legacyImport,
		ConvertImport: opts.Module + "/gen/zengo/convert/" + version,
		RegistrarName: legacyWireEventName(meta.EventService, version),
	}

	for _, rpc := range eventSvc.RPCs {
		rpcData := eventRPCData{
			Name:        rpc.Name,
			RequestType: rpc.RequestType,
		}
		consume := activeLoader.rpcKafkaConsume(legacyImport, eventSvc.Name, rpc.Name)
		if consume != nil {
			rpcData.Consume = &eventConsumeData{
				Topic: consume.Topic,
				Group: consume.Group,
			}
		}
		data.RPCs = append(data.RPCs, rpcData)
	}

	return codegen.Render(codegen.File{
		Template:   "adapter_event.go.tmpl",
		Data:       data,
		OutputPath: filepath.Join(dir, eventAdapterFileName(meta.EventService)),
	})
}
