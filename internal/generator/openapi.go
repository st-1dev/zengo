package generator

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
)

var (
	openAPIVersionDirPattern    = regexp.MustCompile(`^v[0-9]+$`)
	openAPIVersionPrefixPattern = regexp.MustCompile(`^/(v[0-9]+)(?:/|$)`)
	openAPIVersionRPCPattern    = regexp.MustCompile(`\.(v[0-9]+)\.`)
)

// SplitOpenAPIAt rewrites grpc-gateway OpenAPI artifacts into separate
// hub/versioned specs under gen/openapi.
func SplitOpenAPIAt(root string) error {
	dir := filepath.Join(root, "gen", "openapi")

	grouped, err := collectGeneratedOpenAPIGroups(dir)
	if err != nil {
		return err
	}
	if len(grouped) > 0 {
		return rewriteOpenAPIDir(dir, grouped)
	}
	return splitCombinedOpenAPI(dir)
}

func collectGeneratedOpenAPIGroups(dir string) (map[string][]map[string]any, error) {
	apiRoot := filepath.Join(dir, "api")
	entries, err := collectSwaggerFiles(apiRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	grouped := make(map[string][]map[string]any)
	for _, path := range entries {
		rel, err := filepath.Rel(apiRoot, path)
		if err != nil {
			return nil, err
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 2 {
			continue
		}
		group := parts[0]
		if group != "hub" && !openAPIVersionDirPattern.MatchString(group) {
			continue
		}
		var doc map[string]any

		doc, err = readOpenAPIDoc(path)
		if err != nil {
			return nil, err
		}
		if len(readOpenAPIPaths(doc)) == 0 {
			continue
		}
		grouped[group] = append(grouped[group], doc)
	}
	return grouped, nil
}

func collectSwaggerFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	var files []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".swagger.json") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func splitCombinedOpenAPI(dir string) error {
	combinedPath := filepath.Join(dir, "openapi.swagger.json")
	doc, err := readOpenAPIDoc(combinedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	paths := readOpenAPIPaths(doc)
	if len(paths) == 0 {
		return nil
	}

	grouped := make(map[string][]map[string]any)
	for path, item := range paths {
		group := openAPIGroupForPath(path)
		grouped[group] = append(grouped[group], map[string]any{
			"paths": map[string]any{
				rewriteOpenAPIPath(group, path): deepClone(item),
			},
		})
	}

	groupDocs := make(map[string]map[string]any, len(grouped))
	for group, fragments := range grouped {
		out := cloneDocument(doc)
		groupPaths := make(map[string]any, len(fragments))
		for _, fragment := range fragments {
			maps.Copy(groupPaths, readOpenAPIPaths(fragment))
		}
		out["paths"] = groupPaths
		filteredDefinitions := filterDefinitions(doc["definitions"], groupPaths)
		if len(filteredDefinitions) > 0 {
			out["definitions"] = filteredDefinitions
		} else {
			delete(out, "definitions")
		}
		filteredTags := filterTags(doc["tags"], groupPaths)
		if len(filteredTags) > 0 {
			out["tags"] = filteredTags
		} else {
			delete(out, "tags")
		}
		info, ok := out["info"].(map[string]any)
		if ok {
			out["info"] = rewriteInfo(info, group)
		}
		groupDocs[group] = out
	}
	return rewriteOpenAPIDir(dir, groupDocs)
}

func rewriteOpenAPIDir(dir string, grouped any) error {
	docs := make(map[string]map[string]any)
	switch typed := grouped.(type) {
	case map[string]map[string]any:
		docs = typed
	case map[string][]map[string]any:
		for group, items := range typed {
			doc, err := mergeOpenAPIDocs(group, items)
			if err != nil {
				return err
			}
			docs[group] = doc
		}
	default:
		return fmt.Errorf("unsupported openapi grouped payload %T", grouped)
	}

	err := clearDirectory(dir)
	if err != nil {
		return err
	}
	groups := mapsKeys(docs)
	sort.Strings(groups)
	for _, group := range groups {
		body, err := json.MarshalIndent(docs[group], "", "  ")
		if err != nil {
			return err
		}
		body = append(body, '\n')
		err = os.WriteFile(filepath.Join(dir, group+".swagger.json"), body, 0o644)
		if err != nil {
			return err
		}
	}
	return nil
}

func mergeOpenAPIDocs(group string, docs []map[string]any) (map[string]any, error) {
	if len(docs) == 0 {
		return map[string]any{}, nil
	}

	merged := cloneDocument(docs[0])
	paths := map[string]any{}
	definitions := map[string]any{}
	tags := []any{}
	tagNames := map[string]struct{}{}

	for _, doc := range docs {
		for path, item := range readOpenAPIPaths(doc) {
			rewritten := rewriteOpenAPIPath(group, path)
			err := mergeOpenAPIMember(paths, rewritten, item)
			if err != nil {
				return nil, err
			}
		}
		for name, item := range readOpenAPIMap(doc["definitions"]) {
			err := mergeOpenAPIMember(definitions, name, item)
			if err != nil {
				return nil, err
			}
		}
		for _, item := range readOpenAPITags(doc["tags"]) {
			tag, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name, _ := tag["name"].(string)
			if name == "" {
				continue
			}
			_, exists := tagNames[name]
			if exists {
				continue
			}
			tagNames[name] = struct{}{}
			tags = append(tags, deepClone(tag))
		}
	}

	merged["paths"] = paths
	filtered := filterDefinitions(definitions, paths)
	if len(filtered) > 0 {
		merged["definitions"] = filtered
	} else {
		delete(merged, "definitions")
	}
	if len(tags) > 0 {
		merged["tags"] = tags
	} else {
		delete(merged, "tags")
	}
	info, ok := merged["info"].(map[string]any)
	if ok {
		merged["info"] = rewriteInfo(info, group)
	}
	return merged, nil
}

func mergeOpenAPIMember(dst map[string]any, key string, value any) error {
	cloned := deepClone(value)
	existing, ok := dst[key]
	if ok {
		if !reflect.DeepEqual(existing, cloned) {
			return fmt.Errorf("openapi merge conflict for %q", key)
		}
		return nil
	}
	dst[key] = cloned
	return nil
}

func clearDirectory(dir string) error {
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return err
	}
	var entries []os.DirEntry
	entries, err = os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		err := os.RemoveAll(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}

func readOpenAPIDoc(path string) (map[string]any, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	err = json.Unmarshal(body, &doc)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func readOpenAPIPaths(doc map[string]any) map[string]any {
	return readOpenAPIMap(doc["paths"])
}

func readOpenAPIMap(raw any) map[string]any {
	typed, ok := raw.(map[string]any)
	if !ok || len(typed) == 0 {
		return nil
	}
	return typed
}

func readOpenAPITags(raw any) []any {
	typed, ok := raw.([]any)
	if !ok || len(typed) == 0 {
		return nil
	}
	return typed
}

func openAPIGroupForPath(path string) string {
	match := openAPIVersionPrefixPattern.FindStringSubmatch(path)
	if len(match) == 2 {
		return match[1]
	}
	rpcMatch := openAPIVersionRPCPattern.FindStringSubmatch(path)
	if len(rpcMatch) == 2 {
		return rpcMatch[1]
	}
	return "hub"
}

func rewriteOpenAPIPath(group, path string) string {
	if group != "hub" {
		return path
	}
	if strings.HasPrefix(path, "/hub/") || path == "/hub" {
		return path
	}
	return "/hub" + path
}

func cloneDocument(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for key, value := range src {
		switch key {
		case "paths", "definitions", "tags":
			continue
		default:
			out[key] = deepClone(value)
		}
	}
	return out
}

func deepClone(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = deepClone(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, deepClone(item))
		}
		return out
	default:
		return typed
	}
}

func rewriteInfo(info map[string]any, group string) map[string]any {
	out := make(map[string]any, len(info))
	for key, value := range info {
		out[key] = deepClone(value)
	}
	title, _ := out["title"].(string)
	if title == "" {
		title = "openapi"
	}
	out["title"] = title + " (" + group + ")"
	return out
}

func filterDefinitions(raw any, paths map[string]any) map[string]any {
	definitions, ok := raw.(map[string]any)
	if !ok || len(definitions) == 0 {
		return nil
	}

	needed := map[string]struct{}{}
	collectDefinitionRefs(paths, needed)
	queue := make([]string, 0, len(needed))
	for name := range needed {
		queue = append(queue, name)
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		definition, ok := definitions[name]
		if !ok {
			continue
		}
		before := len(needed)
		collectDefinitionRefs(definition, needed)
		if len(needed) == before {
			continue
		}
		for candidate := range needed {
			if !slices.Contains(queue, candidate) {
				queue = append(queue, candidate)
			}
		}
	}

	filtered := make(map[string]any, len(needed))
	for name := range needed {
		definition, ok := definitions[name]
		if ok {
			filtered[name] = deepClone(definition)
		}
	}
	return filtered
}

func collectDefinitionRefs(value any, refs map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		ref, ok := typed["$ref"].(string)
		if ok && strings.HasPrefix(ref, "#/definitions/") {
			refs[strings.TrimPrefix(ref, "#/definitions/")] = struct{}{}
		}
		for _, child := range typed {
			collectDefinitionRefs(child, refs)
		}
	case []any:
		for _, child := range typed {
			collectDefinitionRefs(child, refs)
		}
	}
}

func filterTags(raw any, paths map[string]any) []any {
	tags, ok := raw.([]any)
	if !ok || len(tags) == 0 {
		return nil
	}
	used := map[string]struct{}{}
	collectUsedTags(paths, used)
	if len(used) == 0 {
		return nil
	}

	filtered := make([]any, 0, len(tags))
	for _, item := range tags {
		tag, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := tag["name"].(string)
		_, exists := used[name]
		if exists {
			filtered = append(filtered, deepClone(tag))
		}
	}
	return filtered
}

func collectUsedTags(paths map[string]any, used map[string]struct{}) {
	for _, item := range paths {
		methods, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for _, raw := range methods {
			operation, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			var list []any

			list, ok = operation["tags"].([]any)
			if !ok {
				continue
			}
			for _, tag := range list {
				name, ok := tag.(string)
				if ok {
					used[name] = struct{}{}
				}
			}
		}
	}
}

func mapsKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
