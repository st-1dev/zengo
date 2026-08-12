# Layout: proto, imports, config

Модуль platform: `zengo/platform`. Сервисы живут отдельно (например `examples/user-service`) и подключают platform через `replace` в `go.mod`.

## Дерево platform

```
api/                 # proto: zengo options/manifest + typed config
sdk/                 # runtime: config loader, grpc, kafka, postgres, observability, ...
app/                 # lifecycle приложения
cmd/                 # zengo CLI, protoc-gen-zengo, protoc-gen-go-zz
internal/            # manifest, generator, versioning, scaffold
examples/user-service/
.bin/                # zengo, protoc-gen-zengo (mage zengo)
```

## Typed config: proto → import → файл сервиса

| Config kind | Proto | Go import | Файл в `configs/` | Loader |
|-------------|-------|-----------|-------------------|--------|
| Postgres DB | `api/config/db/postgres/config.proto` | `zengo/platform/api/config/db/postgres` | `postgres.yaml` | `loader.Postgres("postgres")` |
| Kafka | `api/config/queue/kafka/config.proto` | `zengo/platform/api/config/queue/kafka` | `kafka.yaml` | `kafka.BrokersFromLoader(loader, "kafka", ...)` |
| Logging | `api/config/logging/config.proto` | `zengo/platform/api/config/logging` | `logging.yaml` | `loader.Logging("logging")` |
| Tracing | `api/config/tracing/config.proto` | `zengo/platform/api/config/tracing` | `tracing.yaml` | `loader.Tracing("tracing")` |
| Redis | `api/config/db/redis/config.proto` | `zengo/platform/api/config/db/redis` | `redis.yaml` | `loader.Redis(...)` |
| Cassandra | `api/config/db/cassandra/config.proto` | `zengo/platform/api/config/db/cassandra` | `cassandra.yaml` | `loader.Cassandra(...)` |
| S3 | `api/config/storage/s3/config.proto` | `zengo/platform/api/config/storage/s3` | `s3.yaml` | `loader.S3(...)` |

Формат файлов: YAML (`.yaml`/`.yml`) или prototext (`.textproto`/`.pbtxt`). Парсинг: `pkg/sdk/config/configfmt`, загрузка: `sdk/config.Loader`.
Общий TLS schema: `api/config/meta/tls.proto`. Он используется typed config-ами и transport manifest-ом и поддерживает источники TLS-материалов через `path` и `inline_pem`.

## Zengo proto (codegen)

| Proto | Go import | Назначение |
|-------|-----------|------------|
| `api/zengo/options/*.proto` | `zengo/platform/api/zengo/options` | опции для `protoc-gen-zengo` |
| `api/zengo/manifest/*.proto` | `zengo/platform/api/zengo/manifest` | схема `zengo.yaml` / `zengo.textproto` manifest |

Генерация in-place: `mage zengoProto`, `mage configModel` → `zz_generated_*.pb.go` рядом с proto.

## Сервис: что где лежит

| Путь | Назначение |
|------|------------|
| `zengo.textproto` | scaffold default manifest: transports, db, queue, cache/storage, observability |
| `zengo.yaml` | альтернативный YAML manifest, если нужен этот формат |
| `compatibility.legacy_versions` | optional legacy adapter mode: `ENABLED`, `DISABLED`, or omitted (`auto`) |
| `api/` | canonical `hub` proto; `vN` директории optional и используются только для legacy compatibility |
| `gen/` | buf + zengo codegen (`gen/zengo`, `gen/api`, `gen/db`, `gen/openapi`) |
| `internal/<handler>/` | handler, repository |
| `configs/` | typed config файлы |
| `queries/`, `migrations/` | sqlc + postgres; `migrations/001_init.sql` can be generated from hub proto schema |
| `cmd/main.go` | тонкий generated bootstrap поверх `sdk/service`, если файл остаётся generated |
| `docker-compose.yaml` | локальная infra (`mage up`) |

REST transport всегда работает через `grpc-gateway` поверх локального gRPC listener, поэтому `transports.rest` в manifest используется только вместе с `transports.grpc`.
Если `transports.grpc.tls` включён, `transports.rest.grpc_client_tls` задаёт TLS для внутреннего dial из grpc-gateway в gRPC listener.
REST surface разделён на два контура: versioned API (`api/vN/...`) остаётся на своих canonical путях, а hub API монтируется отдельно под `/hub/...`.

Canonical manifest groups:
- `transports`: только `grpc` и `rest`
- `db`: `postgres`, `cassandra`, `oracle`
- `queue`: `kafka`, `nats`, `rabbitmq`
- `cache`: `redis`
- `storage`: `s3`

## CLI

```bash
zengo init [--manifest-format textproto|yaml] <name> [dir]
zengo add kafka grpc redis s3        # patch manifest + templates
zengo gen [--dir DIR] [--proto-only|--wire-only]
zengo check [--dir DIR] [--breaking]
zengo dev [--dir DIR]       # watch + gen + go run
```

Service-level template overrides живут в `.zengo/templates/`. Generator сначала ищет template с тем же именем там, и только потом использует встроенный template из platform.

Новый scaffold создаёт только `api/hub/...`.
Legacy `api/vN/...` layout остаётся compatibility-режимом:
- для новых сервисов он включается через `zengo version freeze <vN>`, что также выставляет `compatibility.legacy_versions: ENABLED`;
- для уже существующих сервисов без этого поля runtime adapters продолжают включаться через auto-detect.

См. также [dev.md](./dev.md).
