package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSplitOpenAPIAtMergesGeneratedVersionedSpecs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "gen", "openapi")
	err := os.MkdirAll(filepath.Join(dir, "api", "hub", "user"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(filepath.Join(dir, "api", "v1", "user"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(filepath.Join(dir, "zengo", "options"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	writeOpenAPIFile(t, filepath.Join(dir, "api", "hub", "user", "service.swagger.json"), map[string]any{
		"swagger": "2.0",
		"info":    map[string]any{"title": "api/hub/user/service.proto"},
		"tags":    []any{map[string]any{"name": "UserService"}},
		"paths": map[string]any{
			"/users": map[string]any{
				"get": map[string]any{
					"tags": []any{"UserService"},
					"responses": map[string]any{
						"200": map[string]any{
							"schema": map[string]any{"$ref": "#/definitions/hubListUsersResponse"},
						},
					},
				},
			},
		},
		"definitions": map[string]any{
			"hubListUsersResponse": map[string]any{
				"properties": map[string]any{
					"status": map[string]any{"$ref": "#/definitions/rpcStatus"},
				},
			},
			"rpcStatus": map[string]any{
				"properties": map[string]any{
					"message": map[string]any{"type": "string"},
				},
			},
		},
	})
	writeOpenAPIFile(t, filepath.Join(dir, "api", "v1", "user", "service.swagger.json"), map[string]any{
		"swagger": "2.0",
		"info":    map[string]any{"title": "api/v1/user/service.proto"},
		"tags":    []any{map[string]any{"name": "UserService"}},
		"paths": map[string]any{
			"/v1/users": map[string]any{
				"get": map[string]any{
					"tags": []any{"UserService"},
					"responses": map[string]any{
						"200": map[string]any{
							"schema": map[string]any{"$ref": "#/definitions/v1ListUsersResponse"},
						},
					},
				},
			},
		},
		"definitions": map[string]any{
			"v1ListUsersResponse": map[string]any{
				"properties": map[string]any{
					"id": map[string]any{"type": "string"},
				},
			},
		},
	})
	writeOpenAPIFile(t, filepath.Join(dir, "zengo", "options", "repository.swagger.json"), map[string]any{
		"swagger":     "2.0",
		"info":        map[string]any{"title": "zengo/options/repository.proto"},
		"paths":       map[string]any{},
		"definitions": map[string]any{"unused": map[string]any{"type": "string"}},
	})
	writeOpenAPIFile(t, filepath.Join(dir, "hub.swagger.json"), map[string]any{
		"swagger": "2.0",
		"paths":   map[string]any{"/stale": map[string]any{}},
	})

	err = SplitOpenAPIAt(root)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(dir, "api"))
	if !os.IsNotExist(err) {
		t.Fatalf("expected raw api directory removed, got %v", err)
	}
	_, err = os.Stat(filepath.Join(dir, "zengo"))
	if !os.IsNotExist(err) {
		t.Fatalf("expected zengo directory removed, got %v", err)
	}

	hub := readOpenAPIFile(t, filepath.Join(dir, "hub.swagger.json"))
	hubPaths := objectField(t, hub, "paths")
	_, ok := hubPaths["/hub/users"]
	if !ok {
		t.Fatalf("hub paths = %#v", hub["paths"])
	}
	hubDefs := objectField(t, hub, "definitions")
	_, ok = hubDefs["hubListUsersResponse"]
	if !ok {
		t.Fatalf("hub definitions = %#v", hubDefs)
	}
	_, ok = hubDefs["rpcStatus"]
	if !ok {
		t.Fatalf("hub definitions missing rpcStatus: %#v", hubDefs)
	}

	v1 := readOpenAPIFile(t, filepath.Join(dir, "v1.swagger.json"))
	v1Paths := objectField(t, v1, "paths")
	_, ok = v1Paths["/v1/users"]
	if !ok {
		t.Fatalf("v1 paths = %#v", v1["paths"])
	}
	_, err = os.Stat(filepath.Join(dir, "openapi.swagger.json"))
	if !os.IsNotExist(err) {
		t.Fatalf("unexpected combined spec left behind: %v", err)
	}
}

func TestSplitOpenAPIAtSeparatesHubAndVersionedSpecs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "gen", "openapi")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{
		"swagger": "2.0",
		"info": map[string]any{
			"title": "demo",
		},
		"tags": []any{
			map[string]any{"name": "HubService"},
			map[string]any{"name": "LegacyService"},
		},
		"paths": map[string]any{
			"/users": map[string]any{
				"get": map[string]any{
					"tags": []any{"HubService"},
					"responses": map[string]any{
						"200": map[string]any{
							"schema": map[string]any{"$ref": "#/definitions/hubUsers"},
						},
					},
				},
			},
			"/v1/users": map[string]any{
				"get": map[string]any{
					"tags": []any{"LegacyService"},
					"responses": map[string]any{
						"200": map[string]any{
							"schema": map[string]any{"$ref": "#/definitions/v1Users"},
						},
					},
				},
			},
		},
		"definitions": map[string]any{
			"hubUsers": map[string]any{
				"properties": map[string]any{
					"child": map[string]any{"$ref": "#/definitions/shared"},
				},
			},
			"v1Users": map[string]any{
				"properties": map[string]any{
					"id": map[string]any{"type": "string"},
				},
			},
			"shared": map[string]any{
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
	}
	var body []byte
	body, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(dir, "openapi.swagger.json"), body, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	err = SplitOpenAPIAt(root)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(dir, "openapi.swagger.json"))
	if !os.IsNotExist(err) {
		t.Fatalf("expected combined spec removed, got %v", err)
	}

	hub := readOpenAPIFile(t, filepath.Join(dir, "hub.swagger.json"))
	hubPaths := objectField(t, hub, "paths")
	_, ok := hubPaths["/hub/users"]
	if !ok {
		t.Fatalf("hub paths = %#v", hub["paths"])
	}
	hubDefs := objectField(t, hub, "definitions")
	_, ok = hubDefs["hubUsers"]
	if !ok {
		t.Fatalf("hub definitions = %#v", hubDefs)
	}
	_, ok = hubDefs["shared"]
	if !ok {
		t.Fatalf("hub definitions missing shared ref: %#v", hubDefs)
	}
	_, ok = hubDefs["v1Users"]
	if ok {
		t.Fatalf("hub definitions unexpectedly contain v1Users: %#v", hubDefs)
	}

	v1 := readOpenAPIFile(t, filepath.Join(dir, "v1.swagger.json"))
	v1Paths := objectField(t, v1, "paths")
	_, ok = v1Paths["/v1/users"]
	if !ok {
		t.Fatalf("v1 paths = %#v", v1["paths"])
	}
	v1Defs := objectField(t, v1, "definitions")
	_, ok = v1Defs["v1Users"]
	if !ok {
		t.Fatalf("v1 definitions = %#v", v1Defs)
	}
	_, ok = v1Defs["hubUsers"]
	if ok {
		t.Fatalf("v1 definitions unexpectedly contain hubUsers: %#v", v1Defs)
	}
}

func readOpenAPIFile(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	err = json.Unmarshal(body, &doc)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func objectField(t *testing.T, doc map[string]any, name string) map[string]any {
	t.Helper()
	value, ok := doc[name].(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", name, doc[name])
	}
	return value
}

func writeOpenAPIFile(t *testing.T, path string, doc map[string]any) {
	t.Helper()
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(path, body, 0o644)
	if err != nil {
		t.Fatal(err)
	}
}
