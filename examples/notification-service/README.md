# notification-service

`notification-service` is a Kafka consumer example. It listens for `user.created`
events from `user-service` and prints a human-readable line to stdout for each
event it receives.

## What it demonstrates

- generated Kafka consumer wiring through `zengo.options.consume`
- a lightweight service that keeps gRPC + REST operational endpoints but does
  not require a database
- build metadata on `GET /buildz` and `./bin/notification-service --version`

## Quick start

```bash
# from platform root
mage zengo
export PATH="$PWD/.bin:$PATH"

cd examples/notification-service
mage build
./bin/notification-service
```

Run `user-service` with Kafka enabled and create a user. The notification
service will print a line similar to:

```text
notification-service: user created id=... email=... name=... display_name=... api_version=hub
```

## Endpoints

- `GET /buildz`
- `GET /livez`
- `GET /readyz`
- `GET /startupz`
- `GET /hub/status`
- `POST /hub/notification.hub.NotificationEventHandler/OnUserCreated`

## Checks

```bash
mage check
```
