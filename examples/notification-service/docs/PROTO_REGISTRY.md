# Proto registry

Service "notification-service" should publish protos to Buf Schema Registry:

```bash
buf push --label notification-service
```

Breaking change detection:

```bash
buf breaking --against '.git#branch=main'
```
