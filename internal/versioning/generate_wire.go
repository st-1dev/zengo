package versioning

import (
	"os"
	"path/filepath"
	"zengo/platform/internal/codegen"
)

type legacyWireTemplateData struct {
	HubImport string
	Service   string
	Versions  []legacyWireVersionData
}

type legacyWireVersionData struct {
	Version       string
	AdapterAlias  string
	AdapterImport string
	LegacyAlias   string
	LegacyImport  string
	TypeName      string
	RegisterFunc  string
}

func generateWire(layout Layout, opts GenerateOptions) error {
	path := filepath.Join(opts.GenDir, "zengo", "legacy_wire_gen.go")
	if len(layout.Legacy) == 0 {
		err := os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	meta := opts.HubMeta
	service := meta.PrimaryService
	data := legacyWireTemplateData{
		HubImport: hubImportPath(opts.Module, meta),
		Service:   service,
	}
	for _, v := range layout.Legacy {
		data.Versions = append(data.Versions, legacyWireVersionData{
			Version:       v,
			AdapterAlias:  "adapter" + v,
			AdapterImport: opts.Module + "/gen/zengo/adapters/" + v,
			LegacyAlias:   "legacy" + v,
			LegacyImport:  legacyImportPath(opts.Module, v, meta),
			TypeName:      adapterTypeName(service, v),
			RegisterFunc:  legacyWireGRPCName(service, v),
		})
	}
	return codegen.Render(codegen.File{
		Template:   "legacy_wire.go.tmpl",
		Data:       data,
		OutputPath: path,
	})
}
