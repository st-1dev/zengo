package main

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"zengo/platform/api/zengo/options"
	"zengo/platform/internal/generator"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func generateSchemaMetadata(gen *protogen.Plugin) error {
	manifestData, err := collectRepositorySchemas(gen.Files)
	if err != nil {
		return err
	}
	g := gen.NewGeneratedFile("zengo/schema_gen.json", "")
	var buf bytes.Buffer

	err = generator.EncodeRepositorySchema(&buf, manifestData)
	if err != nil {
		return err
	}
	_, err = g.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("write schema metadata: %w", err)
	}
	return nil
}

func collectRepositorySchemas(files []*protogen.File) (generator.RepositorySchemaManifest, error) {
	manifestData := generator.RepositorySchemaManifest{Repositories: []generator.RepositorySchema{}}
	tables := map[string]string{}
	for _, file := range files {
		if !file.Generate || isOptionsFile(file) {
			continue
		}
		version := versionFromPackage(string(file.Desc.Package()))
		if !isHubVersion(version) {
			continue
		}
		repositories, err := collectFileRepositorySchemas(file)
		if err != nil {
			return generator.RepositorySchemaManifest{}, err
		}
		for _, repository := range repositories {
			owner, exists := tables[repository.Table]
			if exists {
				return generator.RepositorySchemaManifest{}, fmt.Errorf(
					"duplicate repository table %q declared by %s and %s",
					repository.Table,
					owner,
					repository.Service,
				)
			}
			tables[repository.Table] = repository.Service
			manifestData.Repositories = append(manifestData.Repositories, repository)
		}
	}
	slices.SortFunc(manifestData.Repositories, func(left, right generator.RepositorySchema) int {
		compare := strings.Compare(left.Table, right.Table)
		if compare != 0 {
			return compare
		}
		return strings.Compare(left.Service, right.Service)
	})
	return manifestData, nil
}

func collectFileRepositorySchemas(file *protogen.File) ([]generator.RepositorySchema, error) {
	messageIndex := map[string]*protogen.Message{}
	indexMessages(file.Messages, messageIndex)
	repositories := make([]generator.RepositorySchema, 0, len(file.Services))
	for _, svc := range file.Services {
		if isEventService(svc) {
			continue
		}
		repository, ok := repositoryOption(svc)
		if !ok {
			continue
		}
		modelName := strings.TrimSpace(repository.GetModel())
		if modelName == "" {
			return nil, fmt.Errorf("%s: service %s: repository.model is required", file.Desc.Path(), svc.GoName)
		}
		model := messageIndex[modelName]
		if model == nil {
			return nil, fmt.Errorf(
				"%s: service %s: repository.model %q was not found in the same proto file",
				file.Desc.Path(),
				svc.GoName,
				modelName,
			)
		}
		schema, err := buildRepositorySchema(file, svc, repository, model)
		if err != nil {
			return nil, err
		}
		repositories = append(repositories, schema)
	}
	return repositories, nil
}

func buildRepositorySchema(
	file *protogen.File,
	svc *protogen.Service,
	repository *options.Repository,
	model *protogen.Message,
) (generator.RepositorySchema, error) {
	columns := make([]generator.RepositoryColumn, 0, len(model.Fields))
	primaryKeys := 0
	for _, field := range model.Fields {
		column, skip, err := buildRepositoryColumn(field)
		if err != nil {
			return generator.RepositorySchema{}, fmt.Errorf(
				"%s: model %s field %s: %w",
				file.Desc.Path(),
				model.GoIdent.GoName,
				field.GoName,
				err,
			)
		}
		if skip {
			continue
		}
		if column.PrimaryKey {
			primaryKeys++
		}
		columns = append(columns, column)
	}
	if primaryKeys == 0 {
		return generator.RepositorySchema{}, fmt.Errorf(
			"%s: model %s must declare exactly one primary key",
			file.Desc.Path(),
			model.GoIdent.GoName,
		)
	}
	if primaryKeys > 1 {
		return generator.RepositorySchema{}, fmt.Errorf(
			"%s: model %s declares multiple primary keys",
			file.Desc.Path(),
			model.GoIdent.GoName,
		)
	}
	return generator.RepositorySchema{
		File:    file.Desc.Path(),
		Package: string(file.Desc.Package()),
		Service: svc.GoName,
		Entity:  repository.GetEntity(),
		Table:   repository.GetTable(),
		Model:   model.GoIdent.GoName,
		Columns: columns,
	}, nil
}

func buildRepositoryColumn(field *protogen.Field) (generator.RepositoryColumn, bool, error) {
	columnOption, ok := repositoryColumnOption(field)
	if ok && columnOption.GetIgnore() {
		return generator.RepositoryColumn{}, true, nil
	}
	columnName := toSnakeCase(string(field.Desc.Name()))
	if ok {
		name := strings.TrimSpace(columnOption.GetName())
		if name != "" {
			columnName = name
		}
	}
	sqlType := ""
	if ok {
		sqlType = strings.TrimSpace(columnOption.GetSqlType())
	}
	if sqlType == "" {
		inferredSQLType, err := inferPostgresType(field)
		if err != nil {
			return generator.RepositoryColumn{}, false, err
		}
		sqlType = inferredSQLType
	}
	column := generator.RepositoryColumn{
		ProtoField: string(field.Desc.Name()),
		Name:       columnName,
		SQLType:    sqlType,
	}
	if ok {
		column.PrimaryKey = columnOption.GetPrimaryKey()
		column.Nullable = columnOption.GetNullable()
		column.Unique = columnOption.GetUnique()
		column.DefaultSQL = columnOption.GetDefaultSql()
	}
	return column, false, nil
}

func repositoryOption(svc *protogen.Service) (*options.Repository, bool) {
	hasExtension := proto.HasExtension(svc.Desc.Options(), options.E_Repository)
	if !hasExtension {
		return nil, false
	}
	extension := proto.GetExtension(svc.Desc.Options(), options.E_Repository)
	repository, ok := extension.(*options.Repository)
	if !ok || repository == nil {
		return nil, false
	}
	return repository, true
}

func repositoryColumnOption(field *protogen.Field) (*options.Column, bool) {
	hasExtension := proto.HasExtension(field.Desc.Options(), options.E_Column)
	if !hasExtension {
		return nil, false
	}
	extension := proto.GetExtension(field.Desc.Options(), options.E_Column)
	column, ok := extension.(*options.Column)
	if !ok || column == nil {
		return nil, false
	}
	return column, true
}

func indexMessages(messages []*protogen.Message, index map[string]*protogen.Message) {
	for _, message := range messages {
		index[string(message.Desc.Name())] = message
		index[message.GoIdent.GoName] = message
		index[string(message.Desc.FullName())] = message
		indexMessages(message.Messages, index)
	}
}

func inferPostgresType(field *protogen.Field) (string, error) {
	if field.Desc.IsMap() {
		return "", fmt.Errorf("map fields require zengo.options.column.sql_type")
	}
	if field.Desc.Cardinality() == protoreflect.Repeated {
		return "", fmt.Errorf("repeated fields require zengo.options.column.sql_type")
	}
	kind := field.Desc.Kind()
	switch kind {
	case protoreflect.StringKind:
		return "TEXT", nil
	case protoreflect.BoolKind:
		return "BOOLEAN", nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Uint32Kind,
		protoreflect.Fixed32Kind, protoreflect.Sfixed32Kind:
		return "INTEGER", nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Uint64Kind,
		protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind:
		return "BIGINT", nil
	case protoreflect.FloatKind:
		return "REAL", nil
	case protoreflect.DoubleKind:
		return "DOUBLE PRECISION", nil
	case protoreflect.BytesKind:
		return "BYTEA", nil
	case protoreflect.EnumKind:
		return "", fmt.Errorf("enum fields require zengo.options.column.sql_type")
	case protoreflect.MessageKind, protoreflect.GroupKind:
		if field.Message != nil && string(field.Message.Desc.FullName()) == "google.protobuf.Timestamp" {
			return "TIMESTAMPTZ", nil
		}
		return "", fmt.Errorf("message fields require zengo.options.column.sql_type")
	default:
		return "", fmt.Errorf("unsupported field kind %s requires zengo.options.column.sql_type", kind)
	}
}

func toSnakeCase(value string) string {
	var b strings.Builder
	for index, r := range value {
		if unicode.IsUpper(r) {
			if index > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
