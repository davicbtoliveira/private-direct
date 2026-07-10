# Private Direct

Self-hosted direct messaging backend for Private Direct.

## Development

Run tests:

```sh
go test ./...
```

Run locally:

```sh
PRIVATE_DIRECT_OPERATOR_TOKEN=change-me \
PRIVATE_DIRECT_JWT_SECRET=change-me-too \
go run ./cmd/privatedirect
```
