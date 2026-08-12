# Текущая архитектура `zengo/platform`

Go-модуль: `zengo/platform`  
Версия Go в `go.mod`: `1.26`  
Инструментальные версии в `mise.toml`: Go `1.26.1`, Buf `latest`  
Dev Container image: `mcr.microsoft.com/devcontainers/go:1.26`

## Верхний уровень репозитория

| Путь | Назначение |
|---|---|
| `go.mod`, `go.sum` | Корневой Go-модуль платформы и граф зависимостей |
| `magefile.go` | Mage targets для сборки CLI и protoc plugins, генерации platform/example, `build`, `test`, `tidy`, `check`, `dev`; example binary собирается с `sdk/buildinfo` ldflags |
| `buf.work.yaml` | Buf workspace верхнего уровня |
| `docker-compose.yaml` | Локальный инфраструктурный стек |
| `mise.toml` | Tool versions, PATH на `.bin`, задачи `build-tools`, `check` |
| `.devcontainer/devcontainer.json` | Конфигурация devcontainer, установка `buf`, сборка `zengo` и protoc plugins |
| `.gitignore` | Git ignore правила |
| `api/` | Proto-описания платформы и generated Go для config/options/manifest |
| `app/` | Runtime lifecycle abstraction |
| `cmd/` | Исполняемые программы и protoc plugins |
| `docs/` | Документация репозитория |
| `examples/` | Пример сервиса, собранного на платформе |
| `internal/` | Build-time логика CLI и генераторов |
| `pkg/` | Вспомогательные shared-пакеты |
| `sdk/` | Runtime SDK для сервисов |
| `.idea/` | IDE metadata IntelliJ/GoLand |

## Корневой `magefile.go`

Targets:

| Target | Действие |
|---|---|
| `all` | `gen` + `build` |
| `zengoProto` | `buf generate` по `api/zengo/buf.gen.yaml` |
| `protocGenZengo` | Сборка `./cmd/protoc-gen-zengo` в `./.bin/protoc-gen-zengo` |
| `protocGenGoZz` | Сборка `./cmd/protoc-gen-go-zz` в `./.bin/protoc-gen-go-zz` |
| `protocGenOpenAPIV2` | Сборка `./.bin/protoc-gen-openapiv2` для REST OpenAPI generation |
| `configModel` | `buf generate` по `api/config/buf.gen.yaml` |
| `zengo` | Сборка `./cmd/zengo` в `./.bin/zengo` и подготовка локальных protoc plugins, включая OpenAPI |
| `genPlatform` | `protocGenZengo` + `configModel` + `zengo` |
| `genService` | `buf dep update` + `zengo gen` в `examples/user-service` |
| `gen` | `genPlatform` + `genService` |
| `build` | `go build ./...` + сборка example binary с `sdk/buildinfo.Version` и `sdk/buildinfo.Branch` |
| `test` | `go test ./...` + тесты example |
| `tidy` | `go mod tidy` в корне и в example |
| `check` | `zengo check --breaking` для платформы и example |
| `dev` | `zengo dev` для example |

## `api/` — proto-описания платформы

### Файлы верхнего уровня

| Путь | Назначение |
|---|---|
| `api/buf.yaml` | Buf module для `api/` |
| `api/buf.gen.yaml` | Общий Buf generation template уровня `api/` |
| `api/config/buf.gen.yaml` | Buf generation template для typed config моделей |
| `api/zengo/buf.gen.yaml` | Buf generation template для `zengo` manifest/options |

### `api/config/` — typed config schemas

Каждая директория содержит `config.proto` или `type.proto` и generated Go-файл `zz_generated_*.pb.go`.

| Путь | Go package | Содержимое |
|---|---|---|
| `api/config/db/postgres` | `zengo/platform/api/config/db/postgres` | PostgreSQL config schema |
| `api/config/db/cassandra` | `zengo/platform/api/config/db/cassandra` | Cassandra config schema |
| `api/config/db/oracle` | `zengo/platform/api/config/db/oracle` | Oracle config schema |
| `api/config/db/redis` | `zengo/platform/api/config/db/redis` | Redis config schema |
| `api/config/queue/kafka` | `zengo/platform/api/config/queue/kafka` | Kafka config schema |
| `api/config/queue/nats` | `zengo/platform/api/config/queue/nats` | NATS config schema |
| `api/config/queue/rabbitmq` | `zengo/platform/api/config/queue/rabbitmq` | RabbitMQ config schema |
| `api/config/storage/s3` | `zengo/platform/api/config/storage/s3` | S3 config schema |
| `api/config/logging` | `zengo/platform/api/config/logging` | Logging config schema |
| `api/config/tracing` | `zengo/platform/api/config/tracing` | Tracing config schema |
| `api/config/meta` | `zengo/platform/api/config/meta` | Meta type schema, including shared TLS messages |

### `api/zengo/` — платформенные proto для генерации

| Путь | Go package | Назначение |
|---|---|---|
| `api/zengo/manifest/manifest.proto` | `zengo/platform/api/zengo/manifest;manifestpb` | Схема manifest-файла сервиса: `transports`, transport TLS, `db`, `queue`, `cache`, `storage`, `compatibility` |
| `api/zengo/manifest/zz_generated_manifest.pb.go` | `manifestpb` | Generated Go-код для manifest schema |
| `api/zengo/options/kafka.proto` | `zengo/platform/api/zengo/options;options` | Метод-опции Kafka produce/consume |
| `api/zengo/options/repository.proto` | `zengo/platform/api/zengo/options;options` | Service option для repository metadata |
| `api/zengo/options/convert.proto` | `zengo/platform/api/zengo/options;options` | Field-level option для mapping legacy names |
| `api/zengo/options/zz_generated_kafka.pb.go` | `options` | Generated Go для Kafka options |
| `api/zengo/options/zz_generated_repository.pb.go` | `options` | Generated Go для repository options |
| `api/zengo/options/zz_generated_convert.pb.go` | `options` | Generated Go для convert options |

## `app/` — lifecycle пакет

Пакет: `zengo/platform/app`

| Файл | Назначение |
|---|---|
| `app/app.go` | `Component` interface, `NamedComponent`, `App`, `AddComponent`, `AddCleanup`, `Run`, `shutdown` |

`app.App` используется runtime-компонентами из `sdk/grpc`, `sdk/gateway` и generated `cmd/main.go` сервисов.

## `cmd/` — исполняемые программы

### `cmd/zengo/` — CLI

Файлы:

| Файл | Назначение |
|---|---|
| `cmd/zengo/main.go` | Точка входа CLI, dispatch по subcommand, `runGen` |
| `cmd/zengo/commands.go` | `runInit`, `runVersion` |
| `cmd/zengo/check.go` | `runCheck` |
| `cmd/zengo/dev.go` | `runDev` |
| `cmd/zengo/add.go` | `runAdd` |
| `cmd/zengo/helpers.go` | Вспомогательные функции CLI и генерации; применяют manifest compatibility mode для legacy adapters |

Основные зависимости:

- `internal/cqlgen`
- `internal/generator`
- `internal/manifest`
- `internal/scaffold`
- `internal/versioning`

### Остальные `cmd/*`

| Путь | Назначение | Основные зависимости |
|---|---|---|
| `cmd/protoc-gen-zengo/main.go` | Protoc plugin для генерации `gen/zengo/register_*.pb.go` | `google.golang.org/protobuf/compiler/protogen`, `api/zengo/options` |
| `cmd/protoc-gen-go-zz/main.go` | Wrapper над `protoc-gen-go`, переименовывает `*.pb.go` в `zz_generated_*.pb.go` | `os/exec`, файловые операции |
| `cmd/config-demo/main.go` | Демонстрация загрузчика config | `sdk/config` |

## `sdk/` — runtime SDK

### `sdk/auth/`

| Файл | Назначение |
|---|---|
| `sdk/auth/auth.go` | gRPC unary interceptor для API key auth |
| `sdk/auth/auth_test.go` | Тесты auth interceptor |

### `sdk/cassandra/`

| Файл | Назначение |
|---|---|
| `sdk/cassandra/cassandra.go` | Cassandra client helpers |
| `sdk/cassandra/doc.go` | Package doc |

Основная зависимость: `github.com/gocql/gocql`

### `sdk/config/`

| Файл | Назначение |
|---|---|
| `sdk/config/config.go` | `Loader`, методы `Get`, `Postgres`, `Logging` |
| `sdk/config/kinds.go` | Методы `Tracing`, `Kafka`, `Cassandra`, `Redis`, `Nats`, `S3`, `Oracle`, `RabbitMQ` |

Основные зависимости:

- `pkg/sdk/config/storage`
- `api/config/*`

### `sdk/buildinfo/`

| Файл | Назначение |
|---|---|
| `sdk/buildinfo/buildinfo.go` | Build metadata extraction from `runtime/debug`, JSON handler `/buildz`, stdout printer для `--version` |
| `sdk/buildinfo/doc.go` | Package doc |
| `sdk/buildinfo/buildinfo_test.go` | Тесты нормализации build info и JSON contract |

### `sdk/service/`

| Файл | Назначение |
|---|---|
| `sdk/service/service.go` | Canonical runtime bootstrap: observability init, managed gRPC/REST transports, transport TLS wiring, health/build/metrics endpoints, shutdown hooks |
| `sdk/service/doc.go` | Package doc |

### `sdk/policy/`

| Файл | Назначение |
|---|---|
| `sdk/policy/policy.go` | `Options`, `Executor`, gRPC unary interceptor, HTTP middleware, retry/rate-limit/circuit-breaker/concurrency policies |
| `sdk/policy/doc.go` | Package doc |
| `sdk/policy/policy_test.go` | Unit tests policy executor и transport adapters |

### `sdk/db/`

| Путь | Файлы | Назначение |
|---|---|---|
| `sdk/db/postgres` | `doc.go`, `postgres.go` | PostgreSQL DSN и `pgxpool` helpers |
| `sdk/db/redis` | `doc.go`, `redis.go` | Redis client helpers |
| `sdk/db/oracle` | `oracle.go` | Oracle client helpers |

Основные зависимости:

- `sdk/db/postgres` → `api/config/db/postgres`, `github.com/jackc/pgx/v5`, `github.com/exaring/otelpgx`, `sdk/observability`
- `sdk/db/redis` → `github.com/redis/go-redis/v9`, `github.com/redis/go-redis/extra/redisotel/v9`

### `sdk/gateway/`

| Файл | Назначение |
|---|---|
| `sdk/gateway/gateway.go` | HTTP gateway wrapper, TLS-aware listener, grpc-gateway dial TLS, prefixed route groups, policy middleware |

Основные зависимости:

- `github.com/grpc-ecosystem/grpc-gateway/v2/runtime`
- `sdk/observability`
- generated register funcs

### `sdk/grpc/`

| Файл | Назначение |
|---|---|
| `sdk/grpc/doc.go` | Package doc |
| `sdk/grpc/server.go` | Wrapper над `grpc.Server`, lifecycle-компонент для `app.App`, TLS-aware setup |

### `sdk/health/`

| Файл | Назначение |
|---|---|
| `sdk/health/health.go` | HTTP probe handlers `/livez`, `/readyz`, `/startupz` и lifecycle state |

### `sdk/observability/`

| Файл | Назначение |
|---|---|
| `sdk/observability/doc.go` | Package doc |
| `sdk/observability/observability.go` | `Init`, настройка logging/metrics/tracing |
| `sdk/observability/tracing.go` | OTLP/stdout tracing setup |
| `sdk/observability/grpc.go` | gRPC server/client OpenTelemetry options |
| `sdk/observability/http.go` | HTTP instrumentation helpers |
| `sdk/observability/kafka.go` | Kafka header propagation helpers |
| `sdk/observability/span.go` | Span helpers для DB/messaging |
| `sdk/observability/http_test.go` | HTTP instrumentation tests |
| `sdk/observability/tracing_test.go` | Tracing tests |

Основные зависимости:

- `api/config/logging`
- `api/config/tracing`
- `go.opentelemetry.io/otel/*`
- `github.com/prometheus/client_golang`

### `sdk/queue/`

| Путь | Файлы | Назначение |
|---|---|---|
| `sdk/queue/kafka` | `config.go`, `config_test.go`, `doc.go`, `kafka.go` | Kafka producer/consumer, policy-aware consumer execution и config helpers |
| `sdk/queue/nats` | `doc.go`, `nats.go` | NATS pub/sub wrapper |
| `sdk/queue/rabbitmq` | `rabbitmq.go` | RabbitMQ wrapper and TLS-aware connection options helper |

Основные зависимости:

- `sdk/queue/kafka` → `github.com/IBM/sarama`, `sdk/router`, `sdk/observability`, `sdk/policy`
- `sdk/queue/nats` → `github.com/nats-io/nats.go`

### `sdk/router/`

| Файл | Назначение |
|---|---|
| `sdk/router/router.go` | `EventEnvelope`, helpers для REST/gRPC version parsing |
| `sdk/router/router_test.go` | Тесты router helpers |

### `sdk/storage/s3/`

| Файл | Назначение |
|---|---|
| `sdk/storage/s3/doc.go` | Package doc |
| `sdk/storage/s3/s3.go` | S3 wrapper |

### `sdk/tlsconfig/`

| Файл | Назначение |
|---|---|
| `sdk/tlsconfig/doc.go` | Package doc |
| `sdk/tlsconfig/tlsconfig.go` | Общий builder для client/server TLS и mTLS из typed config и manifest |
| `sdk/tlsconfig/tlsconfig_test.go` | Unit tests для TLS material loading и config assembly |

Основные зависимости:

- `github.com/aws/aws-sdk-go-v2/*`
- `go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws`

## `pkg/` — shared helper packages

### `pkg/sdk/config/configfmt/`

| Файл | Назначение |
|---|---|
| `pkg/sdk/config/configfmt/format.go` | `Unmarshal` для YAML / prototext / pbtxt в `proto.Message` |

### `pkg/sdk/config/storage/`

| Файл | Назначение |
|---|---|
| `pkg/sdk/config/storage/storage.go` | Интерфейс `Storage` |

### `pkg/sdk/config/storage/local/`

| Файл | Назначение |
|---|---|
| `pkg/sdk/config/storage/local/storage.go` | Локальная файловая реализация `Storage` |
| `pkg/sdk/config/storage/local/storage_test.go` | Тесты local storage |

## `internal/` — build-time логика

### `internal/codegen/`

Пакет: `zengo/platform/internal/codegen`

| Файл | Назначение |
|---|---|
| `internal/codegen/codegen.go` | `Render`, `Execute`, `FormatSource`, embed templates, service-level overrides из `.zengo/templates`, `goimports`/`gofmt` formatting |
| `internal/codegen/helpers.go` | Template funcs: `join`, `goName`, `ucFirst`, `isScalarType`, `quote` |

Embedded templates:

| Файл | Используется для |
|---|---|
| `templates/adapter_service.go.tmpl` | legacy service adapters |
| `templates/adapter_event.go.tmpl` | legacy event adapters |
| `templates/convert_messages.go.tmpl` | Hub↔Legacy message converters |
| `templates/legacy_wire.go.tmpl` | GRPC wiring legacy adapters |
| `templates/runtime.go.tmpl` | `gen/zengo/runtime_gen.go` |
| `templates/main.go.tmpl` | generated `cmd/main.go` сервиса |

Основная зависимость: `golang.org/x/tools/imports`

### `internal/cqlgen/`

| Файл | Назначение |
|---|---|
| `internal/cqlgen/generator.go` | Парсинг `.cql` файлов и генерация `queries_gen.go` |

Зависимость: `internal/codegen.FormatSource`

### `internal/generator/`

| Файл | Назначение |
|---|---|
| `internal/generator/main.go` | `GenerateMain`, рендер thin generated bootstrap `internal/codegen/templates/main.go.tmpl` поверх `sdk/service` |
| `internal/generator/doc.go` | Package doc for service artifact generators |
| `internal/generator/main_test.go` | Тесты generated `main.go` |

Основная зависимость: `internal/manifest`, `internal/codegen`

### `internal/manifest/`

| Файл | Назначение |
|---|---|
| `internal/manifest/manifest.go` | `Manifest` и вложенные типы, `Load`, методы доступа к transport/db/queue/observability данным |
| `internal/manifest/yaml.go` | YAML patching helpers и поиск manifest path |
| `internal/manifest/manifest_test.go` | Тесты manifest loading |

Основные зависимости:

- `api/zengo/manifest`
- `pkg/sdk/config/configfmt`
- `sigs.k8s.io/yaml`

### `internal/scaffold/`

| Файл | Назначение |
|---|---|
| `internal/scaffold/scaffold.go` | `InitService`, генерация нового сервиса из embedded templates |
| `internal/scaffold/add.go` | `Add`, patch manifest и добавление сервисных компонентов/конфигов |

Templates в `internal/scaffold/templates/`:

| Путь | Назначение |
|---|---|
| `magefile.go.tmpl`, `go.mod.tmpl`, `README.md.tmpl` | Базовые файлы сервиса |
| `buf.gen.yaml`, `buf.yaml`, `sqlc.yaml` | Генераторные конфиги |
| `docker-compose.yaml.tmpl` | Local infra compose |
| `zengo.yaml`, `zengo.textproto.tmpl` | Manifest templates; scaffold default is `zengo.textproto`, YAML is available via `--manifest-format yaml` |
| `api/hub/_handler_/service.proto.tmpl` | Дополнительный hub proto template |
| `api/v1/_handler_/service.proto.tmpl` | Legacy `v1` proto template, сохраняется для compatibility, но не scaffoldится по умолчанию |
| `internal/_handler_/handler.go.tmpl` | Handler template |
| `internal/_handler_/repository.go.tmpl` | Repository template |
| `configs/*.tmpl` | Config templates для cassandra/kafka/logging/nats/postgres/redis/s3/tracing |
| `migrations/001_init.sql.tmpl` | Legacy SQL migration template kept for reference; managed baseline migrations are now generated from hub proto schema |
| `queries/_handler_.sql.tmpl` | SQL query template |
| `third_party/zengo/options/*.proto` | Proto options в `third_party` |
| `zengo/options/*.proto` | Proto options в директории сервиса |

Основная зависимость: `internal/manifest`

### `internal/versioning/`

Пакет compatibility-генерации адаптеров между canonical `hub` API и legacy `vN`. Для greenfield primary path остаётся `api/hub/...`; compatibility-генерация включается explicit manifest mode-ом или auto-detect для уже существующих legacy layout.

#### Основные типы и metadata

| Файл | Назначение |
|---|---|
| `internal/versioning/types.go` | `Field`, `Message`, `RPC`, `Service`, `Schema` |
| `internal/versioning/meta.go` | `HubMeta`, import-path и naming helpers |
| `internal/versioning/kafka_spec.go` | `kafkaConsumeSpec` |
| `internal/versioning/module.go` | `ReadModule` из `go.mod` |
| `internal/versioning/names.go` | `goName` |

#### Обнаружение layout и freeze

| Файл | Назначение |
|---|---|
| `internal/versioning/discover.go` | `Discover(apiRoot)` и `Layout{APIRoot, Hub, Legacy}` |
| `internal/versioning/freeze.go` | `Freeze(apiRoot, version, module, genDir)` и `transformFrozenProto` |
| `internal/versioning/freeze_proto.go` | Regex для `package` и `go_package` в `.proto` |

#### Загрузка schema и descriptor-ов

| Файл | Назначение |
|---|---|
| `internal/versioning/loader.go` | `Loader`, cache, `NewLoader`, `Schema`, `HubMeta`, descriptor loading |
| `internal/versioning/gengo_schema.go` | `LoadSchemaGengo`, обход `gen/api`, gengo schema helpers |
| `internal/versioning/rawdesc.go` | Извлечение `proto_rawDesc` из generated `.pb.go` через Go AST |
| `internal/versioning/protoreflect.go` | `activeLoader`, `findMessageDescriptor` |

Основные зависимости этого слоя:

- `k8s.io/gengo/parser`
- `k8s.io/gengo/types`
- `google.golang.org/protobuf/reflect/protodesc`
- `google.golang.org/protobuf/reflect/protoreflect`
- `google.golang.org/protobuf/types/descriptorpb`
- `api/zengo/options`

#### Планирование и генерация

| Файл | Назначение |
|---|---|
| `internal/versioning/classify.go` | `FieldMapping`, `MessageConversion`, `RPCConversion`, `ConversionPlan`, `BuildPlan` |
| `internal/versioning/generate.go` | `Generate`, cleanup stale generated files, manual-check orchestration |
| `internal/versioning/generate_convert.go` | Подготовка data для `convert_messages.go.tmpl` |
| `internal/versioning/generate_adapters.go` | Подготовка data для `adapter_service.go.tmpl` |
| `internal/versioning/generate_event.go` | Подготовка data для `adapter_event.go.tmpl` |
| `internal/versioning/generate_wire.go` | Подготовка data для `legacy_wire.go.tmpl` |
| `internal/versioning/generate_runtime.go` | Подготовка data для `runtime.go.tmpl` |
| `internal/versioning/manual.go` | Поиск manual converters в `internal/convert/<version>` |

Основная зависимость генерации: `internal/codegen`

#### Тесты и фикстуры

| Путь | Назначение |
|---|---|
| `internal/versioning/discover_test.go` | Тесты discover и `LoadSchemaGengo` |
| `internal/versioning/freeze_test.go` | Тест freeze |
| `internal/versioning/golden_test.go` | Golden test generated tree |
| `internal/versioning/testdata/golden/user-service/gen/zengo/*` | Golden fixtures для adapters/convert/wire/runtime |

## `examples/user-service/` — пример сервиса

### Конфигурация и исходники

| Путь | Назначение |
|---|---|
| `examples/user-service/go.mod`, `go.sum` | Отдельный Go-модуль example |
| `examples/user-service/magefile.go` | Mage targets для сборки и генерации example |
| `examples/user-service/zengo.yaml` | Example service manifest kept in YAML format; enables `grpc` and `rest` transports together |
| `examples/user-service/buf.yaml`, `buf.gen.yaml`, `buf.lock` | Buf config example; `buf.gen.yaml` also generates `gen/openapi/openapi.swagger.json` |
| `examples/user-service/sqlc.yaml`, `cqlc.yaml` | SQL/CQL codegen config |
| `examples/user-service/api/hub/user/service.proto` | Canonical hub API |
| `examples/user-service/api/v1/user/service.proto` | Legacy `v1` API |
| `examples/user-service/internal/user/handler.go` | User handler |
| `examples/user-service/internal/user/repository.go` | User repository |
| `examples/user-service/cmd/main.go` | Generated service entrypoint; starts local gRPC server and exposes versioned plus hub REST through grpc-gateway under canonical paths and `/hub` |
| `examples/user-service/configs/*.yaml`, `logging.textproto` | Runtime configs |
| `examples/user-service/migrations/001_init.sql` | SQL migration |
| `examples/user-service/queries/user.sql` | SQL query file |
| `examples/user-service/cql/schema.cql`, `user.cql` | CQL schema и queries |
| `examples/user-service/docs/PROTO_REGISTRY.md` | Generated proto registry doc |
| `examples/user-service/.cursor/rules/zengo-service.mdc` | Cursor rules file |

### Generated код example

| Путь | Назначение |
|---|---|
| `examples/user-service/gen/api/hub/user/*.pb.go` | Go/gRPC/gateway код для hub API |
| `examples/user-service/gen/api/v1/user/*.pb.go` | Go/gRPC/gateway код для v1 API |
| `examples/user-service/gen/db/*.go` | SQLC generated DB layer |
| `examples/user-service/gen/cql/queries_gen.go` | CQL generated constants |
| `examples/user-service/gen/openapi/hub.swagger.json` | OpenAPI document for hub routes mounted under `/hub/...` |
| `examples/user-service/gen/openapi/v1.swagger.json` | Public OpenAPI document for versioned REST transport |
| `examples/user-service/gen/zengo/register_*.pb.go` | Generated register funcs от `protoc-gen-zengo` |
| `examples/user-service/gen/zengo/adapters/v1/*.go` | Legacy adapters |
| `examples/user-service/gen/zengo/convert/v1/messages_gen.go` | Message converters |
| `examples/user-service/gen/zengo/legacy_wire_gen.go` | Legacy gRPC wiring |
| `examples/user-service/gen/zengo/runtime_gen.go` | Runtime registration helpers, including separate versioned and hub registration sets plus Kafka policy plumbing |
| `examples/user-service/gen/zengo/options/*.pb.go` | Generated local options Go-files |

## `docs/`

| Файл | Назначение |
|---|---|
| `docs/current_arch.md` | Текущая карта архитектуры репозитория |
| `docs/dev.md` | Документация по dev-процессу |
| `docs/layout.md` | Документация по layout сервиса |

## Зависимости верхнего уровня из `go.mod`

Прямые runtime/build-time зависимости, присутствующие в корневом модуле:

- AWS SDK v2
- `github.com/exaring/otelpgx`
- `github.com/gocql/gocql`
- `github.com/grpc-ecosystem/grpc-gateway/v2`
- `github.com/jackc/pgx/v5`
- `github.com/nats-io/nats.go`
- `github.com/prometheus/client_golang`
- `github.com/redis/go-redis/v9`
- `github.com/IBM/sarama`
- `go.opentelemetry.io/*`
- `google.golang.org/grpc`
- `google.golang.org/protobuf`
- `golang.org/x/tools`
- `k8s.io/gengo`
- `sigs.k8s.io/yaml`
