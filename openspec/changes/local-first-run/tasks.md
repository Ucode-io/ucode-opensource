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

- [x] 2.1 `deploy/docker-compose.yml` with postgres, redis, minio and the four
      services plus the frontend, all bound to `127.0.0.1`
- [x] 2.2 Postgres superuser with `CREATEDB` and `CREATEROLE`; point
      `NODE_POSTGRES_HOST` at it
- [x] 2.3 Init container running the three services' migrations with
      `golang-migrate` (object-builder 73, auth 46, company 90)
- [x] 2.4 Dockerfile per service, multi-arch, with object-builder's WORKDIR
      containing `migrations/postgres`
- [x] 2.5 `deploy/.env.example` with working defaults and no secrets
- [x] 2.6 Frontend image serving a runtime config for the two backend URLs, so
      one image works for every installation
- [x] 2.7 Stop auth panicking when the SMS service has no address — SMS is not
      part of this build and gRPC rejects an empty target
- [x] 2.9 Copy the frontend's file: dependencies before pnpm install — the build
      failed on a missing vendor/react18 manifest
- [x] 2.8 Neutralise the plan-limit checks left in auth; they called the billing
      service that no longer exists and blocked the bootstrap

## 3. CLI

- [x] 3.1 `cmd/ucode` skeleton with cobra: `start`, `stop`, `status`, `logs`, `reset`
- [x] 3.2 Preflight: Docker present and running, required ports free
- [x] 3.3 Materialise `~/.ucode/` with the compose file and a generated `.env`,
      including a per-installation `SECRET_KEY`
- [x] 3.4 Wait for readiness: poll auth-service and the gateway until they answer
- [x] 3.5 First-run bootstrap: `POST /company` with the default admin, store a
      marker so it happens once
- [ ] 3.8 Make the bootstrap survive a partial failure. Verified by hand: the
      company row is written before the admin user, so any later failure leaves
      it behind and every retry then fails with "only one company allowed".
      Either wrap Register in a transaction or have the CLI detect and clear the
      half-created state
- [x] 3.9 Preflight must check ports. Port 5432 was already taken on the test
      machine and compose failed with a raw Docker error
- [x] 3.6 Print credentials and open the browser
- [x] 3.7 `reset` removes volumes and the marker
- [x] 3.10 Ship the migrations as their own image. The compose file mounted them
      from the repository by relative path, so the stack only worked from a
      checkout — `ucode start` from ~/.ucode failed on the very first run
- [x] 3.11 Retry the bootstrap instead of polling for readiness. Docker binds a
      published port the moment a container starts, so a successful TCP connect
      says nothing about the process inside; the first attempt raced auth's own
      gRPC listener

## 4. Demo project

- [ ] 4.1 Decide the demo's shape — a small, obviously-not-real dataset
- [ ] 4.2 Create it through the public API, not by touching the database, so it
      exercises the same path a user would
- [ ] 4.3 Make it idempotent: a second run must not duplicate it

## 5. Proof

- [ ] 5.1 CI job that boots the compose stack, waits, creates a table and a
      record through the API and asserts they come back
- [ ] 5.2 Walk the seven acceptance steps by hand on a clean macOS machine
      (in progress: sign-in and table creation confirmed working)
- [ ] 5.3 Repeat on Linux and on Apple Silicon
- [ ] 5.4 Measure time from command to browser; the target is five minutes
      including image pulls

## 6. Billing residue outside company and gateway

- [x] 6.0 auth and object-builder each kept their own copy of
      `billing_service.proto`, so their code still compiled against a service
      that no longer answers. Removed the copies, regenerated, dropped
      `BillingServiceClient` from both client interfaces, and made
      `user_seat_billing.go` a no-op
- [x] 6.1 Plan-limit check on table creation in
      `object-builder/grpc/service/table.go` — found by a user clicking "create
      table" in the browser, not by any test
- [ ] 6.2 Walk the rest of the acceptance path by hand. Three runtime failures
      so far (SMS dial, api-key limit, table limit) all shared one cause:
      cutting Go code leaves the proto contract, and a dead call stays
      compilable until something actually invokes it

## 7. Footprint

Measured on a laptop: the running stack takes ~304 MB of memory across eight
containers and about 9% CPU at rest. What hurt was building it — repeated Go
compiles, a pnpm install and a Vite build filled 4.4 GB of build cache and
pinned every core. Users must never have to do that.

- [x] 7.4 Set memory limits per service in compose. There are none today, so a
      runaway Postgres or MinIO has nothing stopping it
- [x] 7.5 State the requirements in the README: memory, disk, Docker, and that
      building from source is not part of installing
- [ ] 7.6 Make ghcr.io images the default in the compose file, so `ucode start`
      pulls rather than expecting locally built `:test` tags

## 8. Release

- [ ] 8.1 goreleaser config for darwin and linux, amd64 and arm64
- [ ] 8.2 Publish service images to ghcr.io from CI
- [ ] 8.3 Quickstart in the README that matches what the CLI actually does
