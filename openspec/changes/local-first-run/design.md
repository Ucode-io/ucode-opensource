# Design

## Boot traps that must be fixed first

These were found by reading each service's config loader. All three stop a bare
`.env` from working, and two of them fail silently.

| Where | What happens | Fix |
|---|---|---|
| `services/company/cmd/main.go` | `SECRETS_PROVIDER` defaults to `vault`, `VAULT_ADDRESS` is empty, `NewClient` fails and `main()` **returns** — the process exits with one log line | Ship `SECRETS_PROVIDER=redis` in `.env.example`; the Redis provider already exists at `services/company/pkg/vault/redis_client.go` |
| `services/auth/cmd/main.go` | `HTTP_PORT` has no default, so gin binds `:80`, fails, and the error is discarded by `_ = r.Run(...)`; the process then exits, killing the gRPC listener too | Give `HTTP_PORT` a default, stop discarding the error |
| `services/auth/config` | `AUTH_GRPC_PORT` has no default (fails loudly) | Default to `:9103` |
| `services/company` | `NODE_POSTGRES_HOST` empty, so creating any resource fails | Point at the compose Postgres |

`SECRET_KEY` in auth defaults to the literal `"snZV9XNmvf"`. The CLI must
generate one per installation and write it to the env file.

## Multi-database model

Each project gets its own database and role, created on `NODE_POSTGRES_HOST` by
`CreateUserAndDatabase` in `services/company/pkg/postgres_client`. Locally that
is the single compose Postgres, with a superuser that has `CREATEDB` and
`CREATEROLE`. This is the supported single-node case, not a workaround.

At request time connections resolve through `psqlpool.Get(projectId)` in
`services/object-builder/pool/pool.go`; a miss is a hard error. The pool is
filled by `Register` at provisioning time and by `AutoConnect` on boot.

Object-builder resolves `file://migrations/postgres` relative to its working
directory, so its container's WORKDIR must contain that path.

## Bootstrap

`POST /company` on auth-service already does the whole job in one call:
company → project → environment → API key → admin user → membership →
Postgres resource → 142 migrations into the new database → default roles and
permissions. The CLI calls it; nothing new is written.

The response carries the project and platform ids. The frontend reads
`project_id` from JWT claims, so nothing has to be baked into its build.

## Ports

| Service | Port |
|---|---|
| postgres | 5432 |
| redis | 6379 |
| minio | 9000 |
| object-builder | 7107 (gRPC) |
| auth | 9103 (gRPC), HTTP to be fixed |
| company | 8092 (gRPC) |
| gateway | 8000 (HTTP) |
| admin frontend | 3000 → nginx 80 |

The frontend talks to **two** backends: the gateway for `/v1`, `/v2`, `/v3`, and
auth-service directly for login. Both must be reachable from the browser.

## CLI

A Go binary in `cmd/ucode`, built with the same toolchain as the services and
released through goreleaser for darwin and linux on amd64 and arm64.

State lives in `~/.ucode/`: the compose file, the generated `.env` and a marker
recording whether the first run already happened.

`ucode start` checks Docker, checks the ports, writes the files, pulls images,
brings the stack up, waits for auth-service to answer, bootstraps on first run,
seeds the demo, prints the credentials and opens a browser.

## Default admin

`admin@ucode.local` with a fixed password, printed on every start. It is
acceptable because the stack binds to `127.0.0.1` only. If a future flag exposes
it beyond loopback, refuse to start while the password is still the default.
