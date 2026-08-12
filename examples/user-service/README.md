# user-service

Example Zengo service generated from the platform templates.

## Quick start

```bash
# from platform root
mage zengo

cd examples/user-service
mage up
mage dev
mage build
./bin/user-service --version
```

The example keeps its manifest in `zengo.yaml`, but `zengo.textproto` is supported equally. It explicitly enables legacy `api/v1` adapters through `compatibility.legacy_versions: ENABLED`. Generated services use canonical config kind `postgres`.
Its generated `cmd/main.go` is a thin bootstrap around `sdk/service`.
The example runs `transports.grpc` and `transports.rest` together, and the REST surface is exposed through `grpc-gateway` against the local gRPC listener.
Transport TLS can be added through `transports.grpc.tls`, `transports.rest.tls`, and `transports.rest.grpc_client_tls`. Typed infra configs share the TLS schema from `api/config/meta/tls.proto`.
Public runtime exposes versioned REST routes from `api/v1/...` and hub REST routes separately under `/hub/...`.
`zengo gen` produces split OpenAPI specs in `gen/openapi/`, currently `v1.swagger.json` and `hub.swagger.json`.

If the REST gateway is enabled, the service also exposes build metadata on `GET /buildz`.

## Proto-driven migration baseline

`api/hub/user/service.proto` declares `UserRecord` as the persistence model through `(zengo.options.repository).model = "UserRecord"`.
That dedicated hub-only message drives the managed Postgres baseline migration:

- `(zengo.options.column)` marks the primary key, uniqueness, defaults, and any explicit SQL overrides.
- `zengo gen` writes `migrations/001_init.sql` from the hub persistence schema before running `sqlc generate`.
- This first iteration is baseline-only: later `ALTER` migrations stay manual.
- If you remove the generated header from `migrations/001_init.sql`, `zengo gen` stops rewriting the file.

The public `User` API message can keep fields that are not part of persistence, while `UserRecord` carries persistence-only columns such as `created_at`.

## Custom App Config

The example includes a service-local typed config at [`api/config/app/config.proto`](./api/config/app/config.proto) with data file [`configs/app.yaml`](./configs/app.yaml).

[`internal/user/handler.go`](./internal/user/handler.go) loads it through the shared platform loader when the handler is created, so the generated bootstrap can stay generic while the service keeps its own typed app config.

Current fields:
- `default_display_name_prefix`
- `require_display_name`
