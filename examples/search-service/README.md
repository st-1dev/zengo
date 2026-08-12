# search-service

`search-service` is a read-through cache example. It looks up users in Redis
first and, when the cache is empty, fetches them from `user-service` over gRPC
and stores the result back in Redis.

## What it demonstrates

- versioned public API (`/v1/...`) plus hub API (`/hub/...`)
- Redis-backed read-through cache
- service-to-service gRPC client calls into `user-service`
- local app config for upstream address and cache TTL

## Quick start

```bash
# from platform root
mage zengo
export PATH="$PWD/.bin:$PATH"

cd examples/search-service
mage build
./bin/search-service
```

By default the service expects:

- `user-service` gRPC on `127.0.0.1:9090`
- Redis config in `configs/redis.yaml`
- cache tuning in `configs/app.yaml`

## API

- `GET /v1/users/{id}`
- `GET /v1/users:search?query=alice`
- `GET /hub/users/{id}`
- `GET /hub/users:search?query=alice`
- `GET /buildz`
- `GET /livez`
- `GET /readyz`
- `GET /startupz`

On cache miss, `search-service` calls `user-service`, caches the result, and
serves the response from its own API.

## Checks

```bash
mage check
```
