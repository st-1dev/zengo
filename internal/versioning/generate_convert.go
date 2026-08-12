package versioning

import (
	"path/filepath"
	"sort"
	"zengo/platform/internal/codegen"
)

type convertTemplateData struct {
	Version      string
	HubImport    string
	LegacyImport string
	Messages     []MessageConversion
}

func generateConvert(version string, plan ConversionPlan, opts GenerateOptions) error {
	dir := filepath.Join(opts.GenDir, "zengo", "convert", version)
	data := convertTemplateData{
		Version:      version,
		HubImport:    hubImportPath(opts.Module, opts.HubMeta),
		LegacyImport: legacyImportPath(opts.Module, version, opts.HubMeta),
	}

	for _, msg := range plan.Messages {
		if msg.Mode != "AUTO" {
			continue
		}
		data.Messages = append(data.Messages, msg)
	}
	sort.Slice(data.Messages, func(i, j int) bool {
		return data.Messages[i].LegacyMessage < data.Messages[j].LegacyMessage
	})

	return codegen.Render(codegen.File{
		Template:   "convert_messages.go.tmpl",
		Data:       data,
		OutputPath: filepath.Join(dir, "messages_gen.go"),
	})
}
