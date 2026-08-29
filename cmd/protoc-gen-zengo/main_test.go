package main

import (
	"encoding/json"
	"strings"
	"testing"
	"zengo/platform/api/zengo/options"
	"zengo/platform/internal/generator"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestVersionFromPackage(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"demo.user.hub": "hub",
		"demo.user.v1":  "v1",
		"demo.user":     "v0",
	}
	for in, want := range cases {
		got := versionFromPackage(in)
		if got != want {
			t.Fatalf("versionFromPackage(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRunPlugin_GeneratesSchemaMetadataForHubService(t *testing.T) {
	t.Parallel()
	file := repositoryHubFile(t, repositoryHubFileOptions{withIgnoredRepeated: true})
	response := runPluginForFiles(t, file)

	schemaFile := responseFileByName(t, response, "zengo/schema_gen.json")
	registerFile := responseFileByName(t, response, "zengo/register_hub.pb.go")
	if !strings.Contains(registerFile.GetContent(), "package zengo") {
		t.Fatalf("register_hub.pb.go missing package declaration: %q", registerFile.GetContent())
	}
	var manifestData generator.RepositorySchemaManifest

	err := json.Unmarshal([]byte(schemaFile.GetContent()), &manifestData)
	if err != nil {
		t.Fatalf("unmarshal schema metadata: %v", err)
	}
	if len(manifestData.Repositories) != 1 {
		t.Fatalf("len(repositories) = %d, want 1", len(manifestData.Repositories))
	}
	repository := manifestData.Repositories[0]
	if repository.Table != "users" {
		t.Fatalf("repository.Table = %q", repository.Table)
	}
	if repository.Model != "UserRecord" {
		t.Fatalf("repository.Model = %q", repository.Model)
	}
	if len(repository.Columns) != 3 {
		t.Fatalf("len(columns) = %d, want 3", len(repository.Columns))
	}
	if repository.Columns[0].PrimaryKey != true {
		t.Fatalf("first column is not primary key: %+v", repository.Columns[0])
	}
	if repository.Columns[1].Unique != true {
		t.Fatalf("second column is not unique: %+v", repository.Columns[1])
	}
	if repository.Columns[2].Name != "name" {
		t.Fatalf("third column name = %q", repository.Columns[2].Name)
	}
}

func TestRunPlugin_IgnoresLegacyRepositorySchemas(t *testing.T) {
	t.Parallel()
	file := repositoryV1File(t)
	response := runPluginForFiles(t, file)

	schemaFile := responseFileByName(t, response, "zengo/schema_gen.json")
	var manifestData generator.RepositorySchemaManifest

	err := json.Unmarshal([]byte(schemaFile.GetContent()), &manifestData)
	if err != nil {
		t.Fatalf("unmarshal schema metadata: %v", err)
	}
	if len(manifestData.Repositories) != 0 {
		t.Fatalf("len(repositories) = %d, want 0", len(manifestData.Repositories))
	}
}

func TestRunPlugin_ErrorsOnMissingRepositoryModel(t *testing.T) {
	t.Parallel()
	file := repositoryHubFile(t, repositoryHubFileOptions{omitModel: true})
	_, err := newPluginForFiles(file)
	if err != nil {
		t.Fatalf("newPluginForFiles: %v", err)
	}
	_, err = runPluginForFilesWithError(file)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "repository.model is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPlugin_ErrorsOnInvalidPrimaryKey(t *testing.T) {
	t.Parallel()
	file := repositoryHubFile(t, repositoryHubFileOptions{omitPrimaryKey: true})
	_, err := runPluginForFilesWithError(file)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must declare exactly one primary key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPlugin_ErrorsOnDuplicateTables(t *testing.T) {
	t.Parallel()
	left := repositoryHubFile(t, repositoryHubFileOptions{})
	right := repositoryHubFile(
		t,
		repositoryHubFileOptions{
			fileName:    "api/hub/account/service.proto",
			packageName: "account.hub",
			goPackage:   "example.com/demo/gen/api/hub/account;accounthub",
			serviceName: "AccountService",
			table:       "users",
			entity:      "account",
			model:       "AccountRecord",
		},
	)
	_, err := runPluginForFilesWithError(left, right)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "duplicate repository table") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPlugin_ErrorsOnUnsupportedFieldWithoutSQLType(t *testing.T) {
	t.Parallel()
	file := repositoryHubFile(t, repositoryHubFileOptions{withUnsupportedRepeated: true})
	_, err := runPluginForFilesWithError(file)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "repeated fields require zengo.options.column.sql_type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type repositoryHubFileOptions struct {
	entity                  string
	fileName                string
	goPackage               string
	model                   string
	omitModel               bool
	omitPrimaryKey          bool
	packageName             string
	serviceName             string
	table                   string
	withIgnoredRepeated     bool
	withUnsupportedRepeated bool
}

func repositoryHubFile(t *testing.T, opts repositoryHubFileOptions) *descriptorpb.FileDescriptorProto {
	t.Helper()
	fileName := opts.fileName
	if fileName == "" {
		fileName = "api/hub/user/service.proto"
	}
	packageName := opts.packageName
	if packageName == "" {
		packageName = "user.hub"
	}
	goPackage := opts.goPackage
	if goPackage == "" {
		goPackage = "example.com/demo/gen/api/hub/user;userhub"
	}
	serviceName := opts.serviceName
	if serviceName == "" {
		serviceName = "UserService"
	}
	entity := opts.entity
	if entity == "" {
		entity = "user"
	}
	table := opts.table
	if table == "" {
		table = "users"
	}
	model := opts.model
	if model == "" {
		model = "UserRecord"
	}
	serviceOptions := &descriptorpb.ServiceOptions{}
	repositoryOption := &options.Repository{Entity: entity, Table: table}
	if !opts.omitModel {
		repositoryOption.Model = model
	}
	setServiceRepositoryExtension(t, serviceOptions, repositoryOption)

	fields := []*descriptorpb.FieldDescriptorProto{}
	if !opts.omitPrimaryKey {
		fields = append(fields, stringField(t, "id", 1, &options.Column{PrimaryKey: true}))
	}
	fields = append(fields, stringField(t, "email", 2, &options.Column{Unique: true}))
	fields = append(fields, stringField(t, "name", 3, nil))
	if opts.withIgnoredRepeated {
		fields = append(fields, repeatedStringField(t, "labels", 4, &options.Column{Ignore: true}))
	}
	if opts.withUnsupportedRepeated {
		fields = append(fields, repeatedStringField(t, "aliases", 5, nil))
	}
	modelMessage := &descriptorpb.DescriptorProto{
		Name:  new(model),
		Field: fields,
	}
	apiMessage := &descriptorpb.DescriptorProto{
		Name: new("User"),
		Field: []*descriptorpb.FieldDescriptorProto{
			stringField(t, "id", 1, nil),
		},
	}
	service := &descriptorpb.ServiceDescriptorProto{
		Name:    new(serviceName),
		Options: serviceOptions,
	}
	return &descriptorpb.FileDescriptorProto{
		Name:    new(fileName),
		Package: new(packageName),
		Syntax:  new("proto3"),
		Options: &descriptorpb.FileOptions{GoPackage: new(goPackage)},
		MessageType: []*descriptorpb.DescriptorProto{
			apiMessage,
			modelMessage,
		},
		Service: []*descriptorpb.ServiceDescriptorProto{service},
	}
}

func repositoryV1File(t *testing.T) *descriptorpb.FileDescriptorProto {
	t.Helper()
	serviceOptions := &descriptorpb.ServiceOptions{}
	setServiceRepositoryExtension(t, serviceOptions, &options.Repository{Entity: "user", Table: "users"})
	service := &descriptorpb.ServiceDescriptorProto{
		Name:    new("UserService"),
		Options: serviceOptions,
	}
	return &descriptorpb.FileDescriptorProto{
		Name:        new("api/v1/user/service.proto"),
		Package:     new("user.v1"),
		Syntax:      new("proto3"),
		Options:     &descriptorpb.FileOptions{GoPackage: new("example.com/demo/gen/api/v1/user;userv1")},
		Service:     []*descriptorpb.ServiceDescriptorProto{service},
		MessageType: []*descriptorpb.DescriptorProto{{Name: new("User")}},
	}
}

func runPluginForFiles(t *testing.T, files ...*descriptorpb.FileDescriptorProto) *pluginpb.CodeGeneratorResponse {
	t.Helper()
	response, err := runPluginForFilesWithError(files...)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func runPluginForFilesWithError(files ...*descriptorpb.FileDescriptorProto) (*pluginpb.CodeGeneratorResponse, error) {
	plugin, err := newPluginForFiles(files...)
	if err != nil {
		return nil, err
	}
	err = runPlugin(plugin)
	if err != nil {
		return nil, err
	}
	return plugin.Response(), nil
}

func newPluginForFiles(files ...*descriptorpb.FileDescriptorProto) (*protogen.Plugin, error) {
	request := &pluginpb.CodeGeneratorRequest{ProtoFile: files}
	request.FileToGenerate = make([]string, 0, len(files))
	for _, file := range files {
		request.FileToGenerate = append(request.FileToGenerate, file.GetName())
	}
	return protogen.Options{}.New(request)
}

func responseFileByName(
	t *testing.T,
	response *pluginpb.CodeGeneratorResponse,
	name string,
) *pluginpb.CodeGeneratorResponse_File {
	t.Helper()
	for _, file := range response.File {
		if file.GetName() == name {
			return file
		}
	}
	t.Fatalf("response file %q not found", name)
	return nil
}

func stringField(
	t *testing.T,
	name string,
	number int32,
	column *options.Column,
) *descriptorpb.FieldDescriptorProto {
	t.Helper()
	optionsData := &descriptorpb.FieldOptions{}
	if column != nil {
		setFieldColumnExtension(t, optionsData, column)
	}
	return &descriptorpb.FieldDescriptorProto{
		Name:     new(name),
		Number:   new(number),
		Label:    new(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
		Type:     new(descriptorpb.FieldDescriptorProto_TYPE_STRING),
		JsonName: new(name),
		Options:  optionsData,
	}
}

func repeatedStringField(
	t *testing.T,
	name string,
	number int32,
	column *options.Column,
) *descriptorpb.FieldDescriptorProto {
	t.Helper()
	optionsData := &descriptorpb.FieldOptions{}
	if column != nil {
		setFieldColumnExtension(t, optionsData, column)
	}
	return &descriptorpb.FieldDescriptorProto{
		Name:     new(name),
		Number:   new(number),
		Label:    new(descriptorpb.FieldDescriptorProto_LABEL_REPEATED),
		Type:     new(descriptorpb.FieldDescriptorProto_TYPE_STRING),
		JsonName: new(name),
		Options:  optionsData,
	}
}

func setServiceRepositoryExtension(t *testing.T, opts *descriptorpb.ServiceOptions, repository *options.Repository) {
	t.Helper()
	proto.SetExtension(opts, options.E_Repository, repository)
}

func setFieldColumnExtension(t *testing.T, opts *descriptorpb.FieldOptions, column *options.Column) {
	t.Helper()
	proto.SetExtension(opts, options.E_Column, column)
}
