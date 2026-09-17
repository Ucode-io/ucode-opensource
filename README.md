# ucode

An open-source low-code platform. Define your data model in a browser and get a
working application: tables, relations, views, and permissions down to the
individual record and field.

> **Status: pre-release.** The platform runs locally and the core works, but
> this has not been through a public release yet. Expect rough edges.

## Run it

Docker is required. Everything else comes with the CLI.

```bash
curl -fsSL https://raw.githubusercontent.com/Ucode-io/ucode-opensource/main/install.sh | sh
ucode start
```

### Installing without the pipe

Piping a script into a shell is a reasonable thing to refuse. The script does
three things — downloads one archive, checks its sha256, moves the binary onto
your PATH — and you can read it at [install.sh](install.sh) or do the same by
hand. Replace `darwin_arm64` with your platform: `darwin` or `linux`, `amd64`
or `arm64`.

```bash
base=https://github.com/Ucode-io/ucode-opensource/releases/latest/download
curl -fsSLO $base/ucode_darwin_arm64.tar.gz
curl -fsSLO $base/checksums.txt
grep ' ucode_darwin_arm64.tar.gz$' checksums.txt | shasum -a 256 -c -
tar -xzf ucode_darwin_arm64.tar.gz ucode
sudo mv ucode /usr/local/bin/
```

### What it needs

| | |
|---|---|
| Memory | The stack uses about 300 MB across all eight containers. On macOS, Docker Desktop reserves its own virtual machine on top of that — budget 4 GB free for a comfortable time |
| Disk | ~1.5 GB of images, plus whatever your data grows to |
| CPU | Near idle once running |

You never build anything to install ucode: `ucode start` pulls prebuilt images.
Building from source is only for working on ucode itself, and that is the part
that is heavy.

`ucode start` brings up the whole platform, creates the first admin and opens
the admin panel at <http://127.0.0.1:3000>. First run takes a few minutes while
images download; after that it is about fifteen seconds. An installed CLI stays
on the image version it was released with, so an upgrade is deliberate: install
a newer CLI and the images follow.

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

The CLI is a thin wrapper around Docker Compose, and the compose file it embeds
is the one in this repository. To drive it yourself, clone first:

```bash
git clone https://github.com/Ucode-io/ucode-opensource.git
cd ucode-opensource
cp deploy/.env.example deploy/.env
# set SECRET_KEY to a random value
docker compose --project-directory deploy up
```

This route pulls `:latest` rather than a pinned version, and skips the first
admin the CLI creates for you — you get the stack, not the finished setup.

## What is inside

| | |
|---|---|
| `services/object-builder` | The data engine: dynamic table schemas, CRUD over user-defined tables, permissions at record, field, action and view level |
| `services/auth` | JWT, sessions, roles, OAuth2, API keys |
| `services/company` | Companies, projects, environments, resources |
| `services/gateway` | The only service clients talk to directly |
| `apps/admin` | The admin panel — TypeScript, TanStack Router and Query |
| `services/*/protos/` | The gRPC contracts between services. Each service carries the ones it speaks; `make proto` regenerates the Go bindings from them |
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
make vet              # go vet across every module
make test             # unit tests, no Docker needed
make test-integration # integration tests; starts throwaway Postgres containers
make cli              # build bin/ucode
make images           # build the container images locally
make smoke            # walk the user path against a running stack
make proto            # regenerate the gRPC bindings
make swagger          # regenerate the /swagger docs (needs swag v1.8.9)
```

To run the stack from images you just built rather than the published ones:

```bash
make images
UCODE_VERSION=local ./bin/ucode start
```

The repository is a Go workspace of four modules plus the CLI. `go build ./...`
from the root does **not** work — Go requires a package pattern to start inside
a module — so use `make`, which lists them explicitly.

Integration tests are behind a build tag and start their own database through
testcontainers. Nothing needs configuring; Docker just has to be running.

### Releasing

One tag publishes both halves. `images.yml` builds the six container images and
`release.yml` builds the CLI, and because the CLI pins itself to the matching
image tag, the two cannot drift apart.

```bash
git tag v0.1.0
git push origin v0.1.0
```

`make release-dry-run` builds the archives locally without publishing anything.
The CLI embeds copies of `deploy/docker-compose.yml` and
`deploy/postgres-init.sql` — go:embed cannot read outside its own module — so
`make cli-assets` syncs them and a release refuses to run if they have drifted.

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
