# Local development

## Prerequisites

- Go 1.26+
- [mage](https://magefile.org/)
- [buf](https://buf.build/docs/installation)
- protoc plugins: `protoc-gen-go`, `protoc-gen-go-grpc`, `protoc-gen-grpc-gateway`
- Docker (for `mage up`)

Optional: [mise](https://mise.jdx.dev/) — см. `mise.toml` в корне.

## Platform (once)

```bash
cd /path/to/platform
mage zengo
export PATH="$PWD/.bin:$PATH"
```

## New service

```bash
zengo init my-service
cd my-service
mage up           # postgres + kafka + jaeger
mage dev          # watch api/manifest + regen + go run
mage build
./bin/my-service --version
```

По умолчанию scaffold создаёт `zengo.textproto`. Если нужен YAML manifest:

```bash
zengo init --manifest-format yaml my-service
```

Или из существующего example:

```bash
cd examples/user-service
mage up
mage dev
mage build
./bin/user-service --version
```

Сервисы также отдают build metadata по `GET /buildz`, если у них включён REST transport.
Generated `cmd/main.go` использует `sdk/service` как canonical runtime bootstrap.
REST transport в platform всегда работает через `grpc-gateway` поверх локального gRPC listener, поэтому `transports.rest` используется только вместе с `transports.grpc`.
Transport TLS живёт в manifest:
- `transports.grpc.tls`
- `transports.rest.tls`
- `transports.rest.grpc_client_tls` для grpc-gateway dial в локальный gRPC listener
Typed infra configs используют общий `api/config/meta/tls.proto` и поддерживают `path` и `inline_pem`.
Для сервисов с REST transport `zengo gen` создаёт split OpenAPI specs в `gen/openapi/`, например `v1.swagger.json` и `hub.swagger.json`.
Versioned REST endpoints остаются отдельной public surface. Hub REST монтируется отдельно под `/hub/...` во всех режимах запуска.

Proto-driven Postgres baseline migrations:
- annotate the hub repository service with `(zengo.options.repository).model`
- annotate persistence fields with `(zengo.options.column)` when you need key, uniqueness, defaults, ignores, or explicit SQL types
- `zengo gen` writes `migrations/001_init.sql` only for Postgres services with repository schema metadata
- v1 only manages the baseline `CREATE TABLE` file; later `ALTER` migrations stay manual
- if you remove the generated header from `migrations/001_init.sql`, `zengo gen` stops rewriting that file

## Add components later

```bash
zengo add observability grpc
zengo gen
```

## Checks

```bash
# from platform root
mage check

# from service dir
zengo check --breaking

# from any directory
zengo check --dir ./examples/user-service --skip-test
```

`zengo check` включает `go vet ./...`, freshness generated-кода и `go test ./...`.
`--breaking` сравнивает proto с `.git#branch=main` (настраивается через `--against`).

## Split codegen

```bash
# platform protos + CLI only
mage genPlatform

# example service only
mage genService

# or granular
zengo gen --proto-only
zengo gen --wire-only
zengo gen --skip-sqlc --skip-main
```

Service-level overrides для generator templates можно класть в `.zengo/templates/`. Файл с тем же именем, например `.zengo/templates/main.go.tmpl`, переопределяет встроенный template для рендера внутри этого service root.

Новый scaffold использует только `api/hub/...` как primary API layout. Legacy `api/vN/...` остаётся совместимым режимом для уже существующих сервисов.
Если вы создаёте frozen legacy contract через `zengo version freeze <vN>`, CLI автоматически включает manifest compatibility mode.
Existing services с уже существующим `api/vN/...` продолжают работать и без этого поля за счёт auto-detect.

## Go documentation

Handwritten Go code in this repository uses English GoDoc.

- Every exported package should have a package comment or `doc.go`.
- Every exported type, function, method, constant, and variable should have GoDoc.
- Every exported struct field should describe the behavior or contract of that field.
- Non-trivial internal code should explain intent or behavior, not restate syntax.
- Generated Go files and golden snapshots are excluded from manual documentation work.
- Key public workflows should prefer executable `Example...` tests.

## Devcontainer

Откройте репозиторий в VS Code / Cursor с Dev Containers — `.devcontainer/devcontainer.json` использует Go 1.26 образ, устанавливает `mage` и запускает `mage zengo` при создании.

## Root docker-compose

Корневой `docker-compose.yaml` поднимает полный стек для platform (postgres, kafka, cassandra, redis, nats, minio, jaeger). Сервисный `docker-compose.yaml` (из scaffold) — минимальный набор для одного сервиса.
