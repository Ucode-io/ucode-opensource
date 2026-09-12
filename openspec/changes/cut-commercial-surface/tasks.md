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
- [ ] 3.5 The swagger docs are stale generated artifacts — 19 dead ugen paths in
      the gateway's `docs.go`, 4 in auth's — and there is no working
      regeneration recipe: `swag init` does not reproduce them with the obvious
      flags. Same disease the protos had, and worth the same fix
- [ ] 3.6 `make vet` is red on the gateway: `SetUpAPI` copies a `sync.Mutex` by
      value. Not a live bug — the copy happens once at startup and all 146
      methods take a pointer — but the target is documented in the README and
      no workflow runs it
