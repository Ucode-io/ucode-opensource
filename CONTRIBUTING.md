# Contributing

Thanks for taking an interest. This is the open-source core of ucode; the
platform runs locally and the plumbing works, but it has not been through a
public release yet, so expect rough edges and say so when you find them.

## Getting set up

```bash
make cli && ./bin/ucode start
```

Docker is the only prerequisite. That gives you the whole platform with an
admin account; the README has the details.

For working on the code itself:

```bash
make build            # every service
make vet              # go vet; CI runs it too
make test             # unit tests, no Docker
make test-integration # integration tests; they start their own Postgres
```

`go build ./...` from the root does not work — this is a Go workspace of four
modules, and Go needs a package pattern that starts inside one. Use `make`.

## Before you write code

Anything beyond a small fix starts as a proposal in `openspec/`:

```bash
openspec new change your-change-name
```

Fill in `proposal.md` (why and what), `specs/` (what the system must do) and
`tasks.md` (the steps), then open a pull request with just those. It is much
cheaper to disagree about a proposal than about a branch.

Small fixes — a typo, an obviously wrong condition, a missing nil check — do
not need any of that. Just send the patch.

## What we will not merge

The AI generation module, billing, and third-party integrations were removed
on purpose. Their absence is a product decision, not an oversight. Patches
adding them back will be declined, however well written.

Please also do not add a dependency on a hosted service to the core path. A
developer must be able to run everything on a laptop with nothing but Docker.

## Tests

New behaviour needs a test. Where to put it:

- Pure logic — a plain unit test, no tag, must run without Docker.
- Anything touching the database — behind `//go:build integration`, using
  testcontainers as the existing suites do. Never point a test at a database
  you did not start inside the test.

Never commit a credential, a hostname from your own infrastructure, or a
customer's name. Not in code, not in a fixture, not in a test.

## Commits and pull requests

Explain the why in the commit message; the diff already shows the what. If you
found something surprising along the way, say so — that is often the most
valuable part.

Keep pull requests to one topic.

## Reporting a security problem

Do not open a public issue. See [SECURITY.md](SECURITY.md).
