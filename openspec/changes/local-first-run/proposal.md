# Local first run

## Why

The open-source core now builds and its tests run on throwaway containers, but
nobody outside the team can start it. There is no compose file, no way to create
the first admin, and three services fail or silently exit on a bare `.env`.

Until a developer can go from "never heard of ucode" to a working data model in
minutes, the repository is source code rather than a product, and the launch has
nothing to show.

## What

One command brings the whole platform up on a laptop and opens a browser on a
working project:

```
brew install ucode && ucode start
```

macOS and Linux, amd64 and arm64. Docker is a required prerequisite.

### In scope

- `docker-compose.yml` for Postgres, Redis, MinIO, the four Go services and the
  admin frontend, with images published to ghcr.io.
- A `ucode` CLI (`start`, `stop`, `status`, `logs`, `reset`) that owns the
  compose file, waits for readiness, bootstraps the first admin and opens the
  browser.
- First-run bootstrap through the existing `POST /company` in auth-service,
  which already creates company, project, environment, admin user, the project
  database, its migrations and the default roles.
- A seeded demo project so the first screen is not empty.
- Removing the boot traps that make a bare `.env` fail (see design.md).

### Not in scope

- Publishing to Homebrew and apt. The CLI ships as a binary from GitHub
  Releases first; packaging follows once the flow is proven.
- Kubernetes, Helm or any production deployment.
- Anything cut in the previous phase: AI, billing, third-party integrations.

## Acceptance

A developer on a machine that has never run ucode:

1. Runs one command and reaches a browser without editing any file.
2. Signs in with the credentials the CLI printed.
3. Sees a demo project containing data.
4. Creates a table, adds fields of several types, creates a record, reads it in
   the list, edits it, deletes it.
5. Creates a role, restricts it to that table, and observes the restriction.
6. Runs `ucode stop && ucode start` and finds the data still there.
7. Runs `ucode reset` and gets a clean state.

No console errors from missing endpoints at any step.
