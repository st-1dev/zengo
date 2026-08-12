package versioning

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"zengo/platform/api/zengo/options"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"k8s.io/gengo/parser"
	"k8s.io/gengo/types"
)

// Loader caches gengo universe, schemas, and file descriptors for one gen run.
type Loader struct {
	module string
	genDir string

	importPaths []string
	universe    types.Universe
	sourceDir   map[string]string // importPath -> package directory

	schemas map[string]*Schema // segment "hub" | "v1" | ...
	files   map[string]protoreflect.FileDescriptor
	rawDesc map[string][]byte
}

var loaderCache sync.Map // cacheKey -> *Loader

func cacheKey(module, genDir string, paths []string) string {
	return module + "\x00" + genDir + "\x00" + strings.Join(paths, ",")
}

// NewLoader builds or reuses a loader for all API packages under gen/api.
//
// The loader caches schema information for the hub API and any requested legacy
// versions for the duration of one generation pass.
func NewLoader(module, genDir string, legacyVersions []string) (*Loader, error) {
	segments := append([]string{"hub"}, legacyVersions...)
	var allPaths []string
	for _, seg := range segments {
		paths, err := listGenImportPaths(genDir, seg)
		if err != nil {
			return nil, err
		}
		allPaths = append(allPaths, paths...)
	}
	if len(allPaths) == 0 {
		return nil, fmt.Errorf("no packages under %s/api", genDir)
	}

	key := cacheKey(module, genDir, allPaths)
	cached, ok := loaderCache.Load(key)
	if ok {
		loader, ok := cached.(*Loader)
		if !ok {
			return nil, fmt.Errorf("invalid loader cache entry for %s", key)
		}
		return loader, nil
	}

	l := &Loader{
		module:      module,
		genDir:      genDir,
		importPaths: allPaths,
		sourceDir:   map[string]string{},
		schemas:     map[string]*Schema{},
		files:       map[string]protoreflect.FileDescriptor{},
		rawDesc:     map[string][]byte{},
	}
	err := l.loadGengo()
	if err != nil {
		return nil, err
	}
	for _, seg := range segments {
		err := l.buildSegmentSchema(seg)
		if err != nil {
			return nil, err
		}
	}
	loaderCache.Store(key, l)
	return l, nil
}

func (l *Loader) loadGengo() error {
	b := parser.New()
	for _, imp := range l.importPaths {
		err := b.AddDir(imp)
		if err != nil {
			return fmt.Errorf("gengo add %s: %w", imp, err)
		}
	}
	universe, err := b.FindTypes()
	if err != nil {
		return err
	}
	l.universe = universe
	for _, imp := range l.importPaths {
		pkg := universe.Package(imp)
		if pkg == nil {
			return fmt.Errorf("package %q not in gengo universe", imp)
		}
		l.sourceDir[imp] = pkg.SourcePath
	}
	return nil
}

func (l *Loader) buildSegmentSchema(segment string) error {
	paths, err := listGenImportPaths(l.genDir, segment)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no packages for segment %q", segment)
	}
	schema := &Schema{Messages: map[string]Message{}}
	for _, imp := range paths {
		part, err := l.schemaFromPackage(imp)
		if err != nil {
			return err
		}
		if schema.Package == "" {
			schema.Package = part.Package
		}
		maps.Copy(schema.Messages, part.Messages)
		schema.Services = append(schema.Services, part.Services...)
		schema.Files = append(schema.Files, part.Files...)
	}
	sort.Slice(schema.Services, func(i, j int) bool {
		if schema.Services[i].Name == schema.Services[j].Name {
			return schema.Services[i].File < schema.Services[j].File
		}
		return schema.Services[i].Name < schema.Services[j].Name
	})
	sort.Strings(schema.Files)
	l.schemas[segment] = schema
	return nil
}

func (l *Loader) schemaFromPackage(importPath string) (*Schema, error) {
	pkg := l.universe.Package(importPath)
	if pkg == nil {
		return nil, fmt.Errorf("package %q not found", importPath)
	}
	schema := &Schema{
		Package:  importPath,
		Messages: map[string]Message{},
		Files:    []string{importPath},
	}
	typeNames := make([]string, 0, len(pkg.Types))
	for name := range pkg.Types {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)
	for _, name := range typeNames {
		t := pkg.Types[name]
		if t == nil {
			continue
		}
		switch {
		case isProtoMessage(t):
			msg := l.structToMessage(importPath, t)
			schema.Messages[t.Name.Name] = msg
		case isGRPCServerInterface(t):
			schema.Services = append(schema.Services, interfaceToService(importPath, t))
		}
	}
	return schema, nil
}

func (l *Loader) structToMessage(importPath string, t *types.Type) Message {
	msg := Message{Name: t.Name.Name, File: importPath}
	for _, m := range t.Members {
		if m.Embedded || skipProtoMember(m.Name) {
			continue
		}
		f := memberToField(m)
		legacy, ok := l.fieldMapLegacyName(importPath, t.Name.Name, m.Name)
		if ok {
			f.LegacyName = legacy
		}
		msg.Fields = append(msg.Fields, f)
	}
	return msg
}

// Schema returns the normalized schema for segment, such as "hub" or "v1".
func (l *Loader) Schema(segment string) (*Schema, error) {
	s, ok := l.schemas[segment]
	if !ok {
		return nil, fmt.Errorf("segment %q not loaded", segment)
	}
	return s, nil
}

// HubMeta discovers primary hub service metadata from the loaded schemas.
func (l *Loader) HubMeta() (HubMeta, error) {
	paths, err := listGenImportPaths(l.genDir, "hub")
	if err != nil {
		return HubMeta{}, err
	}
	if len(paths) == 0 {
		return HubMeta{}, fmt.Errorf("no hub packages under %s/api/hub", l.genDir)
	}

	var meta HubMeta
	var hubPath string

	hubPath, err = filepath.Abs(paths[0])
	if err != nil {
		return HubMeta{}, err
	}
	var hubRoot string

	hubRoot, err = filepath.Abs(filepath.Join(l.genDir, "api", "hub"))
	if err != nil {
		return HubMeta{}, err
	}
	var relPath string

	relPath, err = filepath.Rel(hubRoot, hubPath)
	if err != nil {
		return HubMeta{}, err
	}
	meta.RelativePath = filepath.ToSlash(relPath)
	if meta.RelativePath == "." || meta.RelativePath == "" {
		meta.RelativePath = "."
		meta.GoImport = l.module + "/gen/api/hub"
	} else {
		meta.GoImport = l.module + "/gen/api/hub/" + meta.RelativePath
	}
	meta.PackageSuffix = "hub"
	var schema *Schema

	schema, err = l.Schema("hub")
	if err != nil {
		return HubMeta{}, err
	}
	for _, svc := range schema.Services {
		if strings.HasPrefix(svc.Name, "Unsafe") {
			continue
		}
		if strings.HasSuffix(svc.Name, "EventHandler") {
			if meta.EventService == "" {
				meta.EventService = svc.Name
			}
			continue
		}
		if meta.PrimaryService == "" {
			meta.PrimaryService = svc.Name
		}
	}
	if meta.PrimaryService == "" {
		return HubMeta{}, fmt.Errorf("hub: no grpc service found")
	}
	return meta, nil
}

func (l *Loader) fileDescriptor(importPath string) (protoreflect.FileDescriptor, error) {
	fd, ok := l.files[importPath]
	if ok {
		return fd, nil
	}
	raw, err := l.rawDescBytes(importPath)
	if err != nil {
		return nil, err
	}
	var fdp descriptorpb.FileDescriptorProto
	err = proto.Unmarshal(raw, &fdp)
	if err != nil {
		return nil, fmt.Errorf("unmarshal descriptor %s: %w", importPath, err)
	}
	fd, err = protodesc.NewFile(&fdp, nil)
	if err != nil {
		return nil, err
	}
	l.files[importPath] = fd
	return fd, nil
}

func (l *Loader) rawDescBytes(importPath string) ([]byte, error) {
	b, ok := l.rawDesc[importPath]
	if ok {
		return b, nil
	}
	dir := l.sourceDir[importPath]
	if dir == "" {
		return nil, fmt.Errorf("source dir for %q unknown", importPath)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".pb.go") {
			continue
		}
		raw, err := rawDescFromPBGo(filepath.Join(dir, e.Name()))
		if err == nil && len(raw) > 0 {
			l.rawDesc[importPath] = raw
			return raw, nil
		}
	}
	return nil, fmt.Errorf("rawDesc not found in %s", importPath)
}

func (l *Loader) fieldMapLegacyName(importPath, messageName, goFieldName string) (string, bool) {
	desc, err := l.messageDescriptor(importPath, messageName)
	if err != nil {
		return "", false
	}
	var fd protoreflect.FieldDescriptor
	for i := 0; i < desc.Fields().Len(); i++ {
		f := desc.Fields().Get(i)
		if goName(string(f.Name())) == goFieldName {
			fd = f
			break
		}
	}
	if fd == nil {
		return "", false
	}
	ext := proto.GetExtension(fd.Options(), options.E_FieldMap)
	if ext == nil {
		return "", false
	}
	fm, ok := ext.(*options.FieldMap)
	if !ok || fm.GetLegacyName() == "" {
		return "", false
	}
	return fm.GetLegacyName(), true
}

func (l *Loader) messageDescriptor(importPath, messageName string) (protoreflect.MessageDescriptor, error) {
	fd, err := l.fileDescriptor(importPath)
	if err != nil {
		return nil, err
	}
	d := findMessageDescriptor(fd, messageName)
	if d == nil {
		return nil, fmt.Errorf("message %q not found in %s", messageName, importPath)
	}
	return d, nil
}

func (l *Loader) rpcKafkaConsume(importPath, serviceName, rpcName string) *kafkaConsumeSpec {
	fd, err := l.fileDescriptor(importPath)
	if err != nil {
		return nil
	}
	for i := 0; i < fd.Services().Len(); i++ {
		svc := fd.Services().Get(i)
		if string(svc.Name()) != serviceName {
			continue
		}
		for j := 0; j < svc.Methods().Len(); j++ {
			m := svc.Methods().Get(j)
			if string(m.Name()) != rpcName {
				continue
			}
			ext := proto.GetExtension(m.Options(), options.E_Consume)
			if ext == nil {
				return nil
			}
			c, ok := ext.(*options.KafkaConsume)
			if !ok || c.GetTopic() == "" {
				return nil
			}
			return &kafkaConsumeSpec{Topic: c.GetTopic(), Group: c.GetGroup()}
		}
	}
	return nil
}

// InvalidateLoaderCache clears the process-wide loader cache (for tests).
func InvalidateLoaderCache() {
	loaderCache = sync.Map{}
}
