# Local runtime

## ADDED Requirements

### Requirement: One command starts the platform
A developer with Docker installed SHALL bring the whole platform up with a
single command and reach a working browser session without editing any file.

#### Scenario: First run on a clean machine
- **WHEN** a developer runs `ucode start` for the first time
- **THEN** Postgres, Redis, MinIO, the four Go services and the admin frontend start
- **AND** the first admin account is created
- **AND** the credentials are printed to the terminal
- **AND** a browser opens on the admin panel

#### Scenario: Docker is missing
- **WHEN** `ucode start` runs and Docker is unavailable
- **THEN** it exits with a message naming Docker as the prerequisite
- **AND** it does not leave a partial stack behind

#### Scenario: A required port is taken
- **WHEN** a port the stack needs is already bound
- **THEN** the command reports which port and by convention which service wants it
- **AND** exits without starting anything

### Requirement: The stack listens on loopback only
The local stack SHALL bind to `127.0.0.1` so that a default password is never
reachable from another machine.

#### Scenario: Default credentials with a non-loopback bind
- **WHEN** the stack is configured to listen beyond loopback
- **AND** the admin password is still the shipped default
- **THEN** startup is refused with an explanation

### Requirement: State survives a restart
Data created through the admin panel SHALL survive stopping and starting the
stack, and SHALL be discardable on request.

#### Scenario: Restart
- **WHEN** a developer runs `ucode stop` and then `ucode start`
- **THEN** projects, tables and records created earlier are still present

#### Scenario: Reset
- **WHEN** a developer runs `ucode reset`
- **THEN** all local state is removed
- **AND** the next `ucode start` bootstraps from scratch

### Requirement: Services start on a bare configuration
Every service SHALL start with the shipped `.env.example` and no external
secret manager, and SHALL fail loudly rather than silently when it cannot.

#### Scenario: No Vault available
- **WHEN** company-service starts with no Vault address configured
- **THEN** it uses the Redis secret provider and serves requests

#### Scenario: A service cannot bind its port
- **WHEN** a service fails to bind its HTTP port
- **THEN** the failure is logged with the port and the process exits non-zero

### Requirement: The first screen is not empty
A first run SHALL leave the developer looking at a demo project containing data
rather than an empty workspace.

#### Scenario: Demo project after first run
- **WHEN** the developer signs in after the first `ucode start`
- **THEN** a demo project exists with at least one table and sample records
