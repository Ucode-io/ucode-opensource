# Tasks

## 1. Pricing data

- [x] 1.1 Strip the seeded fares, fare items, fare item prices and billing
      periods from migrations 47, 68, 70, 72 and 77. The table definitions stay
      so a fork that wants billing has somewhere to put it
- [x] 1.2 Drop `DEFAULT 'monthly'` from `subscription.billing_period_code`.
      It references `ugen_billing_period`, which is now empty, so a defaulted
      insert would have failed the foreign key
- [x] 1.3 Confirm the migrations still apply — all 90 ran clean against a fresh
      Postgres, and the pricing tables came out empty while `currency` (119
      rows) and `language` (184) survived
- [x] 1.4 Confirm the platform tolerates the empty tables. Ran both real query
      shapes from `project.go` against the stripped schema with a project row
      present: the row comes back, `subscription_info` and `fare_name` NULL
- [x] 1.5 Rebuild the migrate image and confirm no price survives in it
- [ ] 1.6 Verify on a genuinely fresh install. Everything above was checked
      against a throwaway database; `ucode reset && ucode start && make smoke`
      is the real proof, and it destroys whatever is in the local stack, so it
      is the owner's call when to run it
- [ ] 1.7 Decide what to do about the already-published images

## 2. Ugen

- [x] 2.1 auth: the five `/v3/ugen/*` routes, `ugent_auth.go`, `UgenLogin` and
      `ugenLoginForAuthenticatedUser` — plus four more ugen functions in the
      session service, the Google OAuth redirect flow (`pkg/helper/google_oauth.go`,
      orphaned once the ugen routes went: the admin panel signs in through
      `default-login` with `type: "google"`, a different path entirely), and a
      hardcoded user id that skipped every access check in `V2HasAccessUser`.
      That last one was not exploitable — user ids are server-generated, so
      nobody could obtain it — but it has no place in public code. 45 lines of
      project and environment checks now run unconditionally
- [x] 2.2 gateway: the four `/v1/ugen/*` routes, `handlers/v1/ugen.go`,
      `models/ugen.go`. **`ugen_vault.go` was kept and renamed `vault.go`** —
      its handlers are mounted on `/v1/vault/keys`, not under the ugen group:
      it manages environment secrets and is a live feature. Also removed the
      `is_uagen` branch (the typo was in the query parameter) which let a
      caller holding a system token act as another user
- [x] 2.3 company: `ugen_template` storage, grpc service and repo, the seven
      ugen project RPCs, their storage counterparts, and the store wiring
- [x] 2.4 Dropped `is_ugen` from the refresh-login response — two gRPC
      round-trips per refresh for a field the admin panel never read
- [x] 2.5 proto: deleted `ugen_template.proto` and stripped 36 rpcs, 64
      messages and 12 fields from the shared contracts across all four
      services, then regenerated. Removed field numbers are marked `reserved`
      rather than left free to be reused.

      This needed a correction first. The repository root's `proto/` is **not**
      what the checked-in bindings were generated from — regenerating from it
      would have deleted messages the hand-written code uses (`MergeContact`,
      `SetPrice`). `services/*/protos/` is the real source: regenerating all
      904 files from it reproduces them byte for byte. That is now `make proto`,
      and the root `proto/` — a stale duplicate nothing built from — is gone
- [x] 2.6 Schema: gutted the seven migrations that created `ugen_template`,
      `ugen_template_reaction`, `ugen_billing_period`,
      `ugen_currency_rate_cache` and `project.is_ugen`.

      Migration 88 was the trap: it does `ALTER TABLE ugen_template` **and**
      adds `project.per_user_price` / `per_user_currency_id`, which the service
      still reads and writes. Gutting the file wholesale would have broken the
      project table. Only the first statement went
- [x] 2.7 Rebuild, test, smoke — `make build` and `make test` green, all
      migrations apply to a fresh database for both services, 0 ugen tables
      left, and the project insert matches the new column list

## 3. Leftovers from the private repository

- [x] 3.1 Removed 601 files: the `ucode_protos` git-submodule copies (467), the
      stale root `proto/` (117), and the per-service `Dockerfile`, `Makefile`,
      `README.md` and `scripts/` that the monorepo replaced — including READMEs
      that were still GitLab's default template pointing at
      `gitlab.udevs.io/github.com/Ucode-io/...`
- [x] 3.2 Removed a live Yandex Maps API key from `apps/admin`. It is public by
      nature (every page load carries it) but billed to whoever issued it, so
      it became `UCODE_YANDEX_MAPS_API_KEY`, substituted at container start like
      the other runtime values. Without it the MAP and POLYGON editors say maps
      are not configured and the rest of the app works
- [x] 3.3 Removed a staging database address from a document and an internal
      hostname from a dead comment
- [ ] 3.4 Rotate the Yandex Maps key. Both it and the staging address are still
      in the git history (`e3d6a4e` and one other commit), and making the
      repository public publishes the history too. Rotating kills the leaked
      key; the address is an address, with no credential attached
- [x] 3.5 The swagger docs regenerate again, and the dead paths are gone.
      `make swagger` is the recipe: `swag init -g api/api.go -o api/docs
      --parseDependency --parseInternal` per service, with swag pinned to
      v1.8.9 to match the library the services link. `--parseDependency` is
      what the obvious invocation was missing — the annotations name generated
      protobuf types, and without it swag never opens those packages.

      Three things had to be repaired before it would run at all:

      - 72 annotations named a package that the file imports under a different
        alias (`http.Response` in a file that imports it as `status`,
        `auth_service.X` in a file that imports it as `pb`). swag resolves
        through the file's own import block, so they had to match it.
      - 16 annotations named `models.X` for types that live in the genproto
        packages — left from a layout that predates this repository. Repointed
        at the package that declares them, each one checked against what the
        handler actually returns.
      - Three pairs of handlers shared an operation id, which swag treats as a
        hard error. Two were `@ID add user project` / `@ID add user to project`
        (swag takes the first token, so both were `add`); the third was a v2
        and a v3 layout handler that also claimed the same `@Router
        /v2/layout`, a route neither is mounted on. They now carry the paths
        they actually serve.

      What changed in the output: the gateway drops 32 paths and 28
      definitions — ugen, fares, pricing, subscriptions, transactions, the
      GitHub integration, micro-frontend management, send-to-gpt — and gains
      the two real routes (`/v1/api-key/validate`, `/v1/table-list`) it had
      never documented. auth drops `/v3/ugen-register` and gains 7 definitions
      that previously failed to resolve. Neither spec has a dangling `$ref`.

      Not gated in CI: swag names a definition after the shortest unambiguous
      form of its package path, and which of two colliding packages wins the
      short name changes between runs. The spec is equivalent either way, so a
      byte-comparison check would fail at random
- [x] 3.6 `make vet` is green, and a workflow now runs it. Three copies of a
      generated protobuf message — and its embedded mutex — by value:
      `SetUpAPI` took `handlers.Handler` by value (now a pointer, which is what
      the 146 methods and `proxyMiddleware` already wanted), and the three
      `*ForLog` helpers in gateway and company shallow-copied a message to strip
      its secret before logging (now `proto.Clone`, which is also the correct
      deep copy — the shallow one left every pointer field shared with the
      original).

      None of the three was a live bug. What made it worth doing is that
      `.github/workflows/check.yml` now runs `make build vet test` plus
      `cli-assets-check` on every push and pull request, so this class of thing
      cannot go unnoticed again; `make vet` is documented alongside the others
      in the README and CONTRIBUTING.

      Not gated: `gofmt`. 55 files are unformatted — import ordering, four
      missing trailing newlines — and enforcing it would mean a mechanical
      55-file commit first. `make fmt` still does it when someone wants to
