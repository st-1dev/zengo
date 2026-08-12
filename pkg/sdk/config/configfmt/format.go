package configfmt

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"sigs.k8s.io/yaml"
)

var supported = []string{".yaml", ".yml", ".textproto", ".pbtxt"}

// SupportedExtensions returns the file extensions understood by the config formatter.
func SupportedExtensions() []string {
	return append([]string(nil), supported...)
}

// IsSupported reports whether path uses a supported config extension.
func IsSupported(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return slices.Contains(supported, ext)
}

// Unmarshal decodes a protobuf message from YAML or prototext based on path.
func Unmarshal(path string, data []byte, msg proto.Message) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		jsonData, err := yaml.YAMLToJSON(data)
		if err != nil {
			return fmt.Errorf("yaml to json: %w", err)
		}
		return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(jsonData, msg)
	case ".textproto", ".pbtxt":
		return prototext.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(data, msg)
	default:
		return fmt.Errorf("unsupported config format %q", filepath.Ext(path))
	}
}

// UnmarshalMeta decodes only metadata fields from a config payload.
func UnmarshalMeta(path string, data []byte, msg proto.Message) error {
	return Unmarshal(path, data, msg)
}

// Marshal encodes a protobuf message as YAML or prototext based on path.
func Marshal(path string, msg proto.Message) ([]byte, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		jsonData, err := protojson.MarshalOptions{
			Multiline:     true,
			Indent:        "  ",
			UseProtoNames: true,
		}.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("marshal json: %w", err)
		}
		var yamlData []byte

		yamlData, err = yaml.JSONToYAML(jsonData)
		if err != nil {
			return nil, fmt.Errorf("json to yaml: %w", err)
		}
		return yamlData, nil
	case ".textproto", ".pbtxt":
		data, err := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("marshal textproto: %w", err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported config format %q", filepath.Ext(path))
	}
}
