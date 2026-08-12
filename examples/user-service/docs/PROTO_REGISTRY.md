# Proto registry

Service "user-service" should publish protos to Buf Schema Registry:

```bash
buf push --label user-service
```

Breaking change detection:

```bash
buf breaking --against '.git#branch=main'
```
