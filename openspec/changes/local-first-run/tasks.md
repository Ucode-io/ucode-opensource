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
- [x] 3.8 Make the bootstrap survive a partial failure — **hit in the wild**
      on a second laptop 2026-09-12, and it cost a demo. company-service
      registers the project's builder resource on object-builder over gRPC
      while creating the first project, and nothing made it wait for that
      service to listen. On a slower machine it lost the race:

          AddResource -> dial tcp 172.18.0.7:7107: connect: connection refused

      The company row survived, the builder resource did not, and the retry was
      refused with "only one company allowed" — which the CLI read as "already
      set up" and reported as success. The user met a login that failed with
      "user project not found" on an installation that said it was running.

      Two fixes. object-builder now has a healthcheck that proves 7107 answers
      (`nc -z`), and company and gateway wait on `service_healthy` rather than
      on the container merely existing — `depends_on` alone only waits for the
      container to start, which is the trap. And `verifySetup` now signs in as
      the admin before the install is called done, so a half-built setup fails
      loudly instead of quietly.

      Verified: object-builder healthy after 5 checks, company started 2.9s
      behind it, smoke green. Still not transactional — a rollback would be the
      real fix; this makes the failure both unlikely and visible
- [x] 3.10 Company registration now undoes itself when it fails.

      A transaction was not available: `Register` is nine writes across two
      services and two databases, reached over gRPC. What it has instead is
      compensation. Each write records how to undo itself, and a deferred
      handler on a named error return replays the recorded undos in reverse
      when the function returns an error — company, project, environment, api
      key, admin user, the user's membership of the project, and the project's
      own Postgres database. The named return is what makes it hard to forget:
      every existing `return nil, err` triggers it, including the ones whose
      expression used a shadowed `err`, because a return statement assigns the
      named results before deferred calls run.

      Reverse order is not cosmetic — project references company, and
      `user_project` references the user, so undoing the outer thing first
      fails on a foreign key. The compensation also runs on a context derived
      with `context.WithoutCancel`: by the time it runs, the caller has often
      already timed out or hung up, which is frequently what failed the
      registration in the first place. It carries its own 30s deadline, and a
      failed undo logs and continues rather than stranding the steps below it —
      above all the company row, which is the one that blocks a retry.

      Two supporting fixes. `CompanyService.Delete` and
      `EnvironmentService.Delete` in company-service logged their failure and
      returned success, so the rollback could never learn an undo had not
      happened; both now return the error. (That also means a failed delete
      from the admin panel stops reporting 204.)

      Tested: `register_rollback_test.go` covers reverse order, continuing past
      a failed undo, and surviving a cancelled caller context. All three were
      mutation-checked — forward order, `return` instead of `continue`, and
      dropping `WithoutCancel` each turn a test red.

      Not yet proven end to end: forcing the original failure (object-builder
      not listening) against a live stack and watching the retry succeed. That
      needs `ucode reset`, so it belongs with 5.2 and 1.6
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

- [x] 4.1 Decide the demo's shape — a small, obviously-not-real dataset
- [x] 4.2 Create it through the public API, not by touching the database, so it
      exercises the same path a user would
- [x] 4.3 Make it idempotent: a second run must not duplicate it
- [x] 4.4 Stop preflight refusing to start when our own stack holds the ports.
      `ucode start` on a running installation told the user to stop whatever was
      using them — which was ucode
- [x] 4.5 Treat "only one company allowed" as already bootstrapped rather than
      an error. Losing the marker file used to make every later start fail

## 5. Proof

- [x] 5.1 CI job that boots the compose stack, waits, creates a table and a
      record through the API and asserts they come back — green on a real
      runner in 6m49s
- [ ] 5.2 Walk the seven acceptance steps by hand on a clean macOS machine
      (in progress: sign-in and table creation confirmed working)
- [ ] 5.3 Repeat on Linux and on Apple Silicon
- [ ] 5.4 Measure time from command to browser; the target is five minutes
      including image pulls

## 6. Billing residue, and one more cache lie

- [x] 6.3 MinIO is gone from Docker Hub — the whole repository, not just the
      `latest` tag. It kept working locally because a year-old image sat in the
      cache; CI failed on it every single run. Now pulled from quay.io, pinned
      to a release, because a floating tag is exactly what disappeared

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
- [x] 7.6 Make ghcr.io images the default in the compose file, so `ucode start`
      pulls rather than expecting locally built `:test` tags

## 8. Release

- [x] 8.1 goreleaser config for darwin and linux, amd64 and arm64 — `.goreleaser.yaml`
      plus `release.yml` on the same `v*` tag as the images. All four targets
      cross-compile and the version ldflag lands (`ucode version 0.1.0-test`).
      Archive names carry no version, so `/releases/latest/download/` resolves
      and `install.sh` never has to ask the API which tag is current
- [x] 8.2 Publish service images to ghcr.io from CI — workflow written, six
      images, amd64 and arm64. Not yet observed running on a real runner
- [x] 8.4 Watch the first images run — green in 7m33s, six images for amd64 and
      arm64. Emulation was not the problem it was expected to be
- [x] 8.5 Make the six ghcr packages public — done. All six answer HTTP 200
      anonymously for both linux/amd64 and linux/arm64, manifests and layer
      blobs alike. Note the check itself: a bare unauthenticated request to
      ghcr returns 401 for public and private packages alike, because that is
      the registry auth challenge, not a visibility signal. The real check is
      to take an anonymous token from `ghcr.io/token?scope=repository:...:pull`
      and fetch the manifest with it
- [x] 8.3 Quickstart in the README that matches what the CLI actually does —
      the install command was missing entirely; the README opened on `ucode
      start` with no way to obtain the binary. Added `install.sh` (checksum
      verified, no sudo, refuses anything but macOS/Linux on amd64/arm64), the
      hand-rolled equivalent for anyone who will not pipe into a shell, the
      clone the "Without the CLI" route actually needs, and a Releasing section
- [x] 8.7 Pin a released CLI to its own image tag. It followed `latest`, so an
      installed CLI would silently pick up images it had never been run
      against. `v0.1.0` publishes images `0.1.0` and a CLI reporting `0.1.0`;
      dev builds still track `latest`. Covered by cmd/ucode/env_test.go, which
      was mutation-checked
- [ ] 8.8 Sign and notarise the macOS binaries. Unsigned, they are quarantined
      on arrival and macOS refuses to run them: the user is shown "Apple could
      not verify… Move to Trash", with no obvious way forward. Confirmed on a
      second laptop 2026-09-12; `xattr -d com.apple.quarantine` cleared it, but
      that is not something to put in a quickstart. Needs a paid Developer ID
      and a notarytool step in the release workflow
- [x] 8.6 Exercise the default image path end to end — done twice. The
      published `:latest` images were run on this machine (smoke green, digest
      matched ghcr exactly), and `install.sh` was run anonymously against the
      v0.1.0 release: it picked the right platform, verified the checksum and
      produced a CLI reporting 0.1.0, which pins images `:0.1.0` — present for
      all six. Neither `chmod` nor `xattr` was needed: curl does not quarantine
