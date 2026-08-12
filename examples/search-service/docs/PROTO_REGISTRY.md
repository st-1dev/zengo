# Proto registry

Service "search-service" should publish protos to Buf Schema Registry:

```bash
buf push --label search-service
```

Breaking change detection:

```bash
buf breaking --against '.git#branch=main'
```
