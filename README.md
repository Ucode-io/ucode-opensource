# ucode

An open-source low-code platform. Define your data model in a browser and get a
working application: tables, relations, views, and permissions down to the
individual record and field.

> **Status: pre-release.** The platform runs locally and the core works, but
> this has not been through a public release yet. Expect rough edges.

## Run it

Docker is required. Everything else comes with the CLI.

### What it needs

| | |
|---|---|
| Memory | The stack uses about 300 MB across all eight containers. On macOS and Windows, Docker Desktop reserves its own virtual machine on top of that — budget 4 GB free for a comfortable time |
| Disk | ~1.5 GB of images, plus whatever your data grows to |
| CPU | Near idle once running |

You never build anything to install ucode: `ucode start` pulls prebuilt images.
Building from source is only for working on ucode itself, and that is the part
that is heavy.

```bash
ucode start
```

That brings up the whole platform, creates the first admin and opens the admin
panel at <http://127.0.0.1:3000>. First run takes a few minutes while images
download; after that it is about fifteen seconds.

```
Login      admin@ucode.local
Password   UcodeAdmin1!
```

| Command | What it does |
|---|---|
| `ucode start` | Bring the stack up |
| `ucode stop` | Stop it, keep the data |
| `ucode status` | What is running |
| `ucode logs [service]` | Follow the logs |
| `ucode reset` | Stop and delete all local data |

The stack binds to `127.0.0.1` only. Change the admin password before putting
it anywhere else.

### Without the CLI

The CLI is a thin wrapper around Docker Compose. If you would rather drive it
yourself:

```bash
cp deploy/.env.example deploy/.env
# set SECRET_KEY to a random value
docker compose --project-directory deploy up
```

## What is inside

| | |
|---|---|
| `services/object-builder` | The data engine: dynamic table schemas, CRUD over user-defined tables, permissions at record, field, action and view level |
| `services/auth` | JWT, sessions, roles, OAuth2, API keys |
| `services/company` | Companies, projects, environments, resources |
| `services/gateway` | The only service clients talk to directly |
| `apps/admin` | The admin panel — TypeScript, TanStack Router and Query |
| `proto/` | The gRPC contracts between services |
| `cmd/ucode` | The CLI above |

### How a request flows

The browser talks to the gateway on `:8000`, except for sign-in, which goes to
auth-service on `:9104` directly. The gateway validates the token, reads the
project id from its claims, and calls object-builder over gRPC.

Data lives on two levels. Three service databases hold platform metadata —
who signed in, which companies, projects and environments exist. Each project
then gets **its own database**, created when the project is created, holding
the tables the user built and the rows in them. Object-builder picks the right
one per request from the project id.

That is also true locally: one Postgres, one database per project, exactly the
single-node arrangement production uses.

## Development

```bash
make build            # build every service
make test             # unit tests, no Docker needed
make test-integration # integration tests; starts throwaway Postgres containers
make cli              # build bin/ucode
```

The repository is a Go workspace of four modules plus the CLI. `go build ./...`
from the root does **not** work — Go requires a package pattern to start inside
a module — so use `make`, which lists them explicitly.

Integration tests are behind a build tag and start their own database through
testcontainers. Nothing needs configuring; Docker just has to be running.

## What this is not

This is the open-source core. Several things that exist in the hosted product
are deliberately not here: AI application generation, billing and payments, and
every third-party integration. Their absence is intentional — please do not
send patches adding them back.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Specifications for work in progress live
in `openspec/`; a change starts as a proposal there before the code moves.

## License

Apache 2.0. See [LICENSE](LICENSE).
