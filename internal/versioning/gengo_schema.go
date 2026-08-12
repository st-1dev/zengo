package versioning

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"k8s.io/gengo/types"
)

// LoadSchemaGengo loads a segment schema using a shared Loader when available.
func LoadSchemaGengo(module, genDir, segment string) (*Schema, error) {
	if activeLoader != nil {
		return activeLoader.Schema(segment)
	}
	var legacy []string
	if segment != "hub" {
		legacy = []string{segment}
	}
	l, err := NewLoader(module, genDir, legacy)
	if err != nil {
		return nil, err
	}
	return l.Schema(segment)
}

func listGenImportPaths(genDir, segment string) ([]string, error) {
	root := filepath.Join(genDir, "api", segment)
	st, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("generated api root %q: %w", root, err)
	} else if !st.IsDir() {
		return nil, fmt.Errorf("generated api root %q is not a directory", root)
	}

	var paths []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		var entries []os.DirEntry
		entries, err = os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			paths = append(paths, gengoPath(path))
			break
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no generated packages under %s", root)
	}
	sort.Strings(paths)
	return paths, nil
}

func gengoPath(path string) string {
	if filepath.IsAbs(path) {
		wd, err := os.Getwd()
		if err == nil {
			rel, err := filepath.Rel(wd, path)
			if err == nil {
				path = rel
			}
		}
	}
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, ".") {
		return "./" + path
	}
	return path
}

func isProtoMessage(t *types.Type) bool {
	if t.Kind != types.Struct {
		return false
	}
	_, ok := t.Methods["ProtoReflect"]
	return ok
}

func isGRPCServerInterface(t *types.Type) bool {
	if t.Kind != types.Interface {
		return false
	}
	name := t.Name.Name
	if !strings.HasSuffix(name, "Server") {
		return false
	}
	if strings.HasPrefix(name, "Unimplemented") || strings.HasPrefix(name, "Unsafe") {
		return false
	}
	return len(t.Methods) > 0
}

func skipProtoMember(name string) bool {
	switch name {
	case "state", "unknownFields", "sizeCache":
		return true
	default:
		return false
	}
}

func memberToField(m types.Member) Field {
	f := Field{Name: m.Name}
	switch m.Type.Kind {
	case types.Slice:
		f.Repeated = true
		f.Type = gengoTypeName(m.Type.Elem)
	case types.Pointer:
		f.Type = gengoTypeName(m.Type.Elem)
	default:
		f.Type = gengoTypeName(m.Type)
	}
	return f
}

func gengoTypeName(t *types.Type) string {
	if t == nil {
		return ""
	}
	switch t.Kind {
	case types.Builtin:
		return t.Name.Name
	case types.Struct, types.Alias:
		return t.Name.Name
	case types.Pointer, types.Slice:
		return gengoTypeName(t.Elem)
	default:
		if t.Name.Name != "" {
			return t.Name.Name
		}
		return t.String()
	}
}

func interfaceToService(importPath string, t *types.Type) Service {
	svc := Service{Name: strings.TrimSuffix(t.Name.Name, "Server"), File: importPath}
	names := make([]string, 0, len(t.Methods))
	for name := range t.Methods {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		mt := t.Methods[name]
		if strings.HasPrefix(name, "mustEmbed") {
			continue
		}
		req, resp := methodRequestResponse(mt)
		svc.RPCs = append(svc.RPCs, RPC{
			Name:         name,
			RequestType:  req,
			ResponseType: resp,
			Service:      svc.Name,
			File:         importPath,
		})
	}
	return svc
}

func methodRequestResponse(mt *types.Type) (req, resp string) {
	if mt == nil || mt.Signature == nil || len(mt.Signature.Parameters) < 2 {
		return "", ""
	}
	req = gengoTypeName(mt.Signature.Parameters[1])
	if len(mt.Signature.Results) > 0 {
		resp = gengoTypeName(mt.Signature.Results[0])
	}
	return req, resp
}
