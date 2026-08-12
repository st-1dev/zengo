# zengo/platform

`zengo/platform` is a Go platform for scaffolded services with protobuf-first APIs, typed config loading, runtime SDK packages, and the `zengo` CLI.

## Quick start

```bash
mage zengo
export PATH="$PWD/.bin:$PATH"

zengo init my-service
cd my-service
mage dev
mage build
./bin/my-service --version
curl http://127.0.0.1:8080/buildz
```

New services scaffold with `zengo.textproto` by default. Use `zengo init --manifest-format yaml ...` when you want a YAML manifest instead.
`zengo gen` produces split OpenAPI specs under `gen/openapi/`, for example `v1.swagger.json` and `hub.swagger.json`. Hub REST routes are mounted separately under `/hub/...` in every runtime mode.
Service-level template overrides can live under `.zengo/templates/`.
For Postgres services, `zengo gen` also manages a baseline `migrations/001_init.sql` from the hub proto persistence model declared by `(zengo.options.repository).model` and field-level `(zengo.options.column)` annotations. Keep the generated header if you want rewrites; remove it to take ownership manually.
Transport TLS lives in the manifest, and typed infra configs share `api/config/meta/tls.proto` for CA/cert/key material from file paths or inline PEM.

More detail lives in [docs/dev.md](docs/dev.md) and [docs/layout.md](docs/layout.md).
