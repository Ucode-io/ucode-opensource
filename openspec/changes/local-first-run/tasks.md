# Tasks

## 1. Unblock the boot path

- [x] 1.1 Give `HTTP_PORT` a default in auth-service and stop discarding the
      error from `r.Run(...)` in `services/auth/cmd/main.go`
- [x] 1.2 Default `AUTH_GRPC_PORT` to `:9103`
- [x] 1.3 Make company-service usable without Vault: verify the Redis secret
      provider path end to end, and fail with a clear message instead of a bare
      `return` from `main()`
- [x] 1.4 Remove the hardcoded `SECRET_KEY` default in auth-service; require it
      to be set
- [x] 1.5 Fix the nil tracer closer that panics every service at startup when
      Jaeger is unreachable — found while testing 1.3

## 2. Compose

- [ ] 2.1 `deploy/docker-compose.yml` with postgres, redis, minio and the four
      services plus the frontend, all bound to `127.0.0.1`
- [ ] 2.2 Postgres superuser with `CREATEDB` and `CREATEROLE`; point
      `NODE_POSTGRES_HOST` at it
- [ ] 2.3 Init container running the three services' migrations with
      `golang-migrate` (object-builder 73, auth 46, company 90)
- [ ] 2.4 Dockerfile per service, multi-arch, with object-builder's WORKDIR
      containing `migrations/postgres`
- [ ] 2.5 `deploy/.env.example` with working defaults and no secrets
- [ ] 2.6 Frontend image serving a runtime config for the two backend URLs, so
      one image works for every installation

## 3. CLI

- [ ] 3.1 `cmd/ucode` skeleton with cobra: `start`, `stop`, `status`, `logs`, `reset`
- [ ] 3.2 Preflight: Docker present and running, required ports free
- [ ] 3.3 Materialise `~/.ucode/` with the compose file and a generated `.env`,
      including a per-installation `SECRET_KEY`
- [ ] 3.4 Wait for readiness: poll auth-service and the gateway until they answer
- [ ] 3.5 First-run bootstrap: `POST /company` with the default admin, store a
      marker so it happens once
- [ ] 3.6 Print credentials and open the browser
- [ ] 3.7 `reset` removes volumes and the marker

## 4. Demo project

- [ ] 4.1 Decide the demo's shape — a small, obviously-not-real dataset
- [ ] 4.2 Create it through the public API, not by touching the database, so it
      exercises the same path a user would
- [ ] 4.3 Make it idempotent: a second run must not duplicate it

## 5. Proof

- [ ] 5.1 CI job that boots the compose stack, waits, creates a table and a
      record through the API and asserts they come back
- [ ] 5.2 Walk the seven acceptance steps by hand on a clean macOS machine
- [ ] 5.3 Repeat on Linux and on Apple Silicon
- [ ] 5.4 Measure time from command to browser; the target is five minutes
      including image pulls

## 6. Release

- [ ] 6.1 goreleaser config for darwin and linux, amd64 and arm64
- [ ] 6.2 Publish service images to ghcr.io from CI
- [ ] 6.3 Quickstart in the README that matches what the CLI actually does
