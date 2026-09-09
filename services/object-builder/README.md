# Ucode Go Object Builder Service.

Ucode's Go Object Builder Service is a gRPC microservice that builds dynamic business objects, relations, and spreadsheet-like payloads for downstream services. It orchestrates Postgres-backed metadata, external service integrations, and object builder pipelines exposed via protobuf APIs.

## Features

- gRPC endpoints generated from the `object_builder_service` proto suite
- Multi-tenant PostgreSQL connection pool keyed by project ID
- Dynamic object composition (tables, views, relations, permissions, AG Grid trees, Excel exports)
- Integration with Auth, Company, Transcoder, and Document Generator services via dedicated gRPC clients
- MinIO-backed file exports plus configurable Jaeger tracing and structured Zap logging

## Architecture Overview

- `cmd/main.go` wires configuration, logging, tracing, database repositories, and gRPC servers.
- `grpc/service` contains service handlers that translate protobuf requests into repository calls.
- `storage/postgres` hosts domain repositories (e.g., `object_builder.go`) and orchestrates SQL generation with pgx & Squirrel.
- `pool` maintains a keyed pgx pool per project for strict data isolation.
- `models` represents schema-specific DTOs marshalled to/from protobuf structs.
- `pkg/*` packages host shared utilities: logging, helper functions, scripts, JS parsing, jaeger, etc.
- `protos` & `genproto` contain protobuf definitions and generated code; `ucode_protos` is tracked as a submodule to sync upstream APIs.

```
┌─────────────┐     ┌───────────────────┐     ┌─────────────────────┐
│ gRPC Client │ --> │ grpc/service/*    │ --> │ storage/postgres/*  │
└─────────────┘     └────────┬──────────┘     └──────────┬──────────┘
                              │                           │
                              ▼                           ▼
                        pkg/helper, models,         pool (pgx), MinIO,
                        tracing, config            external services
```

## Project Layout

| Path | Purpose |
| --- | --- |
| `cmd/` | Entry point that bootstraps the service |
| `config/` | Environment loading and shared constants |
| `grpc/` | gRPC server setup, interceptors, and service registration |
| `grpc/client/` | Outbound gRPC client factories for peer services |
| `storage/postgres/` | Repository implementations and SQL orchestration |
| `models/` | Static Go structs mirroring table schemas and builder payloads |
| `pkg/` | Shared utilities (logger, helper, jaeger, scripts, security, etc.) |
| `pool/` | Per-project pgx pool registry and error helpers |
| `protos/`, `genproto/` | Source protos and generated Go code (do not edit generated files) |
| `migrations/postgres/` | Schema migrations managed via `golang-migrate` |
| `scripts/gen_proto.sh` | Helper for regenerating Go stubs from proto files |

## Requirements

- Go `1.23+`
- PostgreSQL `14+`
- `protoc` + Go plugins (`protoc-gen-go`, `protoc-gen-go-grpc`)
- `golang-migrate` CLI (for `make migration-up`)
- Docker (optional, for container builds)
- Access to upstream `ucode_protos` submodule (ensure Git credentials)

## Getting Started

```bash
git clone git@github.com:ucode-team/ucode_go_object_builder_service.git
cd ucode_go_object_builder_service
cp .env.example .env    # create if you do not already have one
go mod download
```

> There is no committed `.env.example`. Create `.env` manually following the config table below.

### Configuration

The service loads env vars via `config.Load()`. Common keys:

| Variable | Description | Default |
| --- | --- | --- |
| `SERVICE_NAME` | Logical service identifier | `ucode` |
| `OBJECT_BUILDER_SERVICE_HOST`, `OBJECT_BUILDER_SERVICE_PORT` | Listener address | `localhost`, `:7107` |
| `ENVIRONMENT` | `debug`, `test`, `release` | `debug` |
| `JAEGER_URL` | Jaeger agent host:port | empty |
| `POSTGRES_HOST` / `POSTGRES_PORT` / `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DATABASE` | Primary metadata DB | _required_ |
| `POSTGRES_MAX_CONNECTIONS` | Per pool connection ceiling | `500` |
| `AUTH_SERVICE_HOST` / `AUTH_GRPC_PORT` | Auth service target | `localhost` / `:9103` |
| `COMPANY_SERVICE_HOST` / `COMPANY_GRPC_PORT` | Company service target | `localhost` / `:8092` |
| `TRANSCODER_SERVICE_HOST` / `TRANSCODER_GRPC_PORT` | Transcoder service target | `localhost` / `:9110` |
| `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_SSL` | MinIO storage | empty / `true` |
| `NODE_TYPE`, `K8S_NAMESPACE` | Deployment metadata | `LOW`, `cp-region-type-id` |

### Running Locally

```bash
# start the service
make run              # or: go run cmd/main.go

# lint & test
make linter
go test ./...
```

The server listens on `OBJECT_BUILDER_SERVICE_PORT` (default `:7107`). Use grpcurl or integration tests to exercise endpoints defined in `protos/object_builder_service/*.proto`.

### Database

Migrations live under `migrations/postgres`. Point the make target to your DSN before using:

```bash
migrate -path ./migrations/postgres \
  -database "postgres://user:pass@localhost:5432/object_builder?sslmode=disable" \
  up
```

### Proto & Client Code Generation

`ucode_protos` tracks the authoritative proto definitions. Typical workflow:

```bash
make pull-proto-module     # init / sync submodule
make copy-proto-module     # refresh ./protos from submodule
make gen-proto-module      # regenerate ./genproto (requires protoc plugins)
```

Generated code under `genproto/` should not be edited manually.

### Docker

Build and tag an image targeting Linux:

```bash
make build               # produces ./bin/ucode_go_object_builder_service
REGISTRY=registry.example PROJECT_NAME=ucode make build-image
```

## Observability & Operations

- **Tracing**: Jaeger instrumentation is enabled via `opentracing` and `pkg/jaeger`. Set `JAEGER_URL` to your agent address.
- **Logging**: Structured logging uses Zap via `pkg/logger`, with log level tied to `ENVIRONMENT`.
- **Metrics**: Not provided by default; integrate with your stack via interceptors if needed.

## Contributing & Support

- Follow [`CONTRIBUTING.md`](CONTRIBUTING.md) for workflow guidance.
- Be excellent to each other per [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).
- Security issues: email `security@ucode.dev`.
- General questions: `opensource@ucode.dev`.
 