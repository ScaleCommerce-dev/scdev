# scdev Implementation Plan

**Created:** January 2026
**Status:** Implementation Phase - Milestone 12 Complete

---

## Table of Contents

1. [Technical Decisions](#technical-decisions)
2. [Project Structure](#project-structure)
3. [Milestone Overview](#milestone-overview)
4. [Milestone Details](#milestone-details)
5. [Testing Strategy](#testing-strategy)
6. [Development Workflow](#development-workflow)

---

## Technical Decisions

### Language & Runtime

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go 1.22+ | Single binary, good Docker ecosystem, cross-platform |
| CLI Framework | [Cobra](https://github.com/spf13/cobra) | Industry standard, used by Docker/Kubernetes/Hugo |
| Config Parsing | [Viper](https://github.com/spf13/viper) | Works well with Cobra, supports YAML |
| Container Runtime | Shell out to CLI | Future Podman compatibility, easier debugging |
| State Storage | YAML file | Simple, human-readable, no dependencies |

### External Tools (Downloaded on First Run)

| Tool | Purpose | Download Source |
|------|---------|-----------------|
| just | Task runner | GitHub releases |
| mkcert | SSL certificates | GitHub releases |
| ctop | Container monitoring | GitHub releases |
| mutagen | File synchronization | GitHub releases |

### Container Runtime Abstraction

```
┌─────────────────────────────────────┐
│           scdev commands            │
└─────────────────┬───────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│      runtime.Runtime interface      │
│  - CreateContainer()                │
│  - StartContainer()                 │
│  - StopContainer()                  │
│  - RemoveContainer()                │
│  - Exec()                           │
│  - CreateNetwork()                  │
│  - ...                              │
└─────────────────┬───────────────────┘
                  │
        ┌─────────┴─────────┐
        ▼                   ▼
┌───────────────┐   ┌───────────────┐
│ DockerCLI     │   │ PodmanCLI     │
│ (Milestone 1) │   │ (Future)      │
└───────────────┘   └───────────────┘
```

---

## Project Structure

```
scdev/
├── cmd/                        # CLI commands (Cobra)
│   ├── root.go                 # Root command, global flags (--config)
│   ├── version.go              # scdev version
│   ├── start.go                # scdev start
│   ├── stop.go                 # scdev stop
│   ├── down.go                 # scdev down [-v]
│   ├── exec.go                 # scdev exec <service> <command>
│   ├── config.go               # scdev config (show resolved config)
│   ├── list.go                 # scdev list
│   ├── info.go                 # scdev info [--raw]
│   ├── volumes.go              # scdev volumes [--global] [project]
│   ├── cleanup.go              # scdev cleanup [--global] [--force]
│   ├── logs.go                 # scdev logs [service] [-f] [--tail]
│   ├── restart.go              # scdev restart
│   ├── mutagen.go              # scdev mutagen status/reset/flush
│   └── services.go             # scdev services start/stop/status/recreate
│
├── internal/                   # Private application code
│   ├── config/                 # Configuration handling
│   │   ├── config.go           # Config structs
│   │   ├── defaults.go         # All defaults (domain, images, tool versions)
│   │   ├── loader.go           # Config loading, variable substitution
│   │   └── templates/          # Embedded templates with ${VAR} placeholders
│   │       └── global-config.yaml
│   │
│   ├── runtime/                # Container runtime abstraction
│   │   ├── runtime.go          # Interface definition
│   │   ├── docker.go           # Docker CLI implementation
│   │   ├── types.go            # Shared types (Container, Network, etc.)
│   │   └── docker_test.go
│   │
│   ├── project/                # Project management
│   │   └── project.go          # Project operations (start, stop, etc.)
│   │
│   ├── state/                  # State management
│   │   └── state.go            # State file (~/.scdev/state.yaml)
│   │
│   ├── services/               # Shared services
│   │   ├── manager.go          # Shared service lifecycle
│   │   ├── router.go           # Traefik specific
│   │   ├── mail.go             # Mailpit specific
│   │   ├── adminer.go          # Adminer container config + server list generation
│   │   ├── redis_insights.go   # Redis Insights specific
│   │   └── observability.go    # OpenObserve specific
│   │
│   ├── ssl/                    # Certificate management
│   │   └── mkcert.go           # mkcert wrapper
│   │
│   ├── mutagen/                # Mutagen file sync
│   │   └── mutagen.go          # Mutagen wrapper (daemon, sessions)
│   │
│   ├── tools/                  # External tool management
│   │   ├── tools.go            # Download and manage tools
│   │   ├── just.go             # Just specific
│   │   ├── mkcert.go           # mkcert specific
│   │   └── mutagen.go          # Mutagen specific
│   │
│   └── ui/                     # Terminal UI helpers (future)
│       ├── output.go           # Colored output, spinners
│       └── markdown.go         # Markdown rendering
│
├── pkg/                        # Public packages (if any)
│
├── planning/                   # Planning documents (this file)
│   ├── scdev-project-briefing.md
│   └── implementation-plan.md
│
├── testdata/                   # Test fixtures
│   └── projects/
│       ├── minimal/
│       │   └── .scdev/config.yaml
│       └── full/
│           └── .scdev/config.yaml
│
├── go.mod
├── go.sum
├── main.go                     # Entry point
├── Makefile                    # Build commands
└── README.md
```

---

## Milestone Overview

| # | Milestone | Description | Key Deliverable | Status |
|---|-----------|-------------|-----------------|--------|
| 0 | Project Skeleton | Go project setup, CLI framework | `scdev version` works | ✅ Done |
| 1 | Single Container | Basic container lifecycle | Start/stop one container | ✅ Done |
| 2 | Config-Driven | Full config parsing, multi-service | Start project from config | ✅ Done |
| 3 | Networking | Project networks, service DNS | Services can communicate | ✅ Done |
| 4 | Volumes | Named volumes, persistence | Data survives restarts | ✅ Done |
| 5 | Project State | State tracking, list/info | `scdev list` shows projects | ✅ Done |
| 6 | Shared Router | Traefik integration | Access via `*.scdev.local` | ✅ Done |
| 7 | SSL & First-Run | mkcert, setup flow | HTTPS works | ✅ Done |
| 8 | Shared Mail | Mailpit integration | Catch emails | ✅ Done |
| 9 | Justfile | Dynamic commands | `scdev setup/test/cmd` work | ✅ Done |
| 10 | Shared DB UI | Adminer integration | Browse databases | ✅ Done |
| 11 | Mutagen File Sync | Fast file sync on macOS | `scdev mutagen status/reset/flush` | ✅ Done |
| 12 | Shared Redis UI | Redis Insights integration | Browse Redis databases | ✅ Done |

---

## Milestone Details

### Milestone 0: Project Skeleton ✅

**Goal:** Establish Go project structure and basic CLI

**Tasks:**
- [x] Initialize Go module (`go mod init github.com/[user]/scdev`)
- [x] Set up Cobra CLI structure
- [x] Create `scdev version` command
- [x] Create basic project structure (directories)
- [x] Set up Makefile with build targets
- [x] Create config struct (empty, just structure)
- [x] Add first unit test

**Acceptance Criteria:**
```bash
# Build succeeds
make build

# Version command works
./scdev version
# Output: scdev version 0.1.0 (darwin/arm64)

# Help shows available commands
./scdev --help
```

**Files to Create:**
- `main.go`
- `cmd/root.go`
- `cmd/version.go`
- `go.mod`
- `Makefile`
- `internal/config/config.go` (struct only)

---

### Milestone 1: Single Container Lifecycle ✅

**Goal:** Start, stop, and remove a single container via CLI

**Tasks:**
- [x] Define `runtime.Runtime` interface
- [x] Implement `runtime.DockerCLI` (basic operations)
- [x] Create container types (`Container`, `ContainerConfig`)
- [x] Implement `scdev start` (hardcoded single container)
- [x] Implement `scdev stop`
- [x] Implement `scdev down`
- [x] Add integration tests (require Docker)

**Acceptance Criteria:**
```bash
# Start a test container
./scdev start
# Creates and starts: app.test.scdev (nginx:alpine)

# Stop the container
./scdev stop
# Container stopped but exists

# Start again (reuses container)
./scdev start
# Container started

# Remove container
./scdev down
# Container removed
```

**Key Code:**

```go
// internal/runtime/runtime.go
type Runtime interface {
    CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error)
    StartContainer(ctx context.Context, id string) error
    StopContainer(ctx context.Context, id string) error
    RemoveContainer(ctx context.Context, id string) error
    ContainerExists(ctx context.Context, name string) (bool, error)
    ContainerRunning(ctx context.Context, name string) (bool, error)
}
```

---

### Milestone 2: Configuration-Driven Containers ✅

**Goal:** Parse config file and start multiple services

**Tasks:**
- [x] Implement config file parsing (`.scdev/config.yaml`)
- [x] Implement variable substitution (`${PROJECTNAME}`, `${PROJECTDIR}`, `${SCDEV_DOMAIN}`, etc.)
- [x] Support multiple services in config
- [x] Pass environment variables to containers
- [x] Implement `scdev exec <service> <command>`
- [x] Project discovery (find `.scdev/` from current dir)
- [x] Implement `scdev config` (show resolved config)
- [x] Two-pass config parsing for `PROJECTNAME` resolution
- [x] User-friendly config error messages with line numbers

**Acceptance Criteria:**
```bash
# Given .scdev/config.yaml exists with app + db services
cd myproject

# Start reads config and creates containers
./scdev start
# Creates: app.myproject.scdev, db.myproject.scdev

# Exec into container
./scdev exec app sh
# Opens shell in app container

# Show resolved config
./scdev config
# Prints YAML with all variables expanded
```

**Config Example:**
```yaml
version: 1
name: ${PROJECTDIR}  # Uses directory name, or set custom name
domain: ${PROJECTNAME}.${SCDEV_DOMAIN}  # PROJECTNAME is resolved after parsing name field

services:
  app:
    image: node:20-alpine
    environment:
      NODE_ENV: development
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_PASSWORD: postgres
```

**Variable Resolution:**
- `${PROJECTDIR}` - Always the directory basename (available immediately)
- `${PROJECTNAME}` - The resolved `name:` field value (available after first parse pass)
- `${SCDEV_DOMAIN}` - From env var, global config, or default (`scalecommerce.site`)

---

### Milestone 3: Networking ✅

**Goal:** Create project network, enable service-to-service communication

**Tasks:**
- [x] Add network operations to `runtime.Runtime` interface
- [x] Create project network on start (`<projectname>.scdev`)
- [x] Attach all project containers to network
- [x] Add network aliases for service names
- [x] Remove network on `scdev down`

**Acceptance Criteria:**
```bash
./scdev start
# Network created: myproject.scdev
# Containers attached to network

# From app container, can reach db
./scdev exec app ping db
# Resolves and pings successfully

./scdev down
# Network removed
```

---

### Milestone 4: Volumes ✅

**Goal:** Named volume support

**Tasks:**
- [x] Add volume operations to `runtime.Runtime`
- [x] Parse volume config from YAML
- [x] Create named volumes on start
- [x] Mount volumes to containers
- [x] Implement `scdev down -v` (remove volumes)
- [x] Implement `scdev volumes` (list volumes)
- [x] Implement `scdev cleanup` (interactive volume removal)

**Acceptance Criteria:**
```bash
./scdev start
# Volumes created: db_data.myproject.scdev

./scdev down
# Volumes kept

./scdev down -v
# All volumes removed

./scdev volumes
# Lists project volumes

./scdev cleanup
# Interactive cleanup with confirmation
```

**Config Example:**
```yaml
services:
  db:
    image: postgres:16-alpine
    volumes:
      - db_data:/var/lib/postgresql/data

volumes:
  db_data:
```

---

### Milestone 5: Project State ✅

**Goal:** Track projects, show status and info

**Tasks:**
- [x] Create `~/.scdev/` directory structure
- [x] Implement state file (`~/.scdev/state.yaml`)
- [x] Register project on first start
- [x] Unregister project on `scdev down`
- [x] Implement `scdev list` (all projects with status)
- [x] Implement `scdev info` (current project details)
- [x] Show container status (running/stopped/not created)
- [x] Markdown rendering for info output (with `--raw` flag)
- [x] Implement `scdev volumes --global` with orphan detection

**Acceptance Criteria:**
```bash
# After starting a project
./scdev list
# NAME        STATUS     PATH
# myproject   running    /home/user/myproject
# other       stopped    /home/user/other

./scdev info
# Shows: name, domain, services, volumes, status
# Renders markdown from config.info
```

**State File:**
```yaml
# ~/.scdev/state.yaml
projects:
  myproject:
    path: /home/user/myproject
    last_started: 2025-01-23T14:30:00Z
  other:
    path: /home/user/other
    last_started: 2025-01-22T09:00:00Z
```

---

### Milestone 6: Shared Router (Traefik) ✅

**Goal:** Shared Traefik instance routes traffic to projects

**Tasks:**
- [x] Create global config (`~/.scdev/global-config.yaml`) - auto-created from embedded template
- [x] Implement shared services manager (`internal/services/manager.go`)
- [x] Create shared network (`scdev_shared`)
- [x] Start Traefik container with proper config
- [x] Connect Traefik to project networks on start
- [x] Disconnect on project stop/down
- [x] Implement multi-protocol routing config (http, https, tcp, udp)
- [x] Implement `scdev services start/stop/status`

**Multi-Protocol Routing:**

Instead of manual Traefik labels, services use a simplified `routing` block:

```yaml
services:
  app:
    image: node:20-alpine
    routing:
      protocol: http    # http, https, tcp, or udp
      port: 3000        # Container port (default: 80 for http, 443 for https)

  db:
    image: mysql:8.0
    routing:
      protocol: tcp
      port: 3306        # Container port
      host_port: 3306   # Host port (required for tcp/udp)

  syslog:
    image: syslog-ng
    routing:
      protocol: udp
      port: 514
      host_port: 5514
```

Traefik labels are automatically generated based on the routing config. TCP/UDP services require the router to be recreated with the new ports, which happens automatically on `scdev start`.

**Acceptance Criteria:**
```bash
# Start shared services
./scdev services start
# Traefik running on ports 80, 443

# Start project
./scdev start
# Traefik connected to project network

# Access via hostname
curl http://myproject.scalecommerce.site
# Routes to project's app container

./scdev services status
# Shows: router (running), mail (stopped), etc.
```

---

### Milestone 7: SSL & First-Run ✅

**Goal:** HTTPS with valid certificates, smooth first-run experience

**Tasks:**
- [x] Implement tool downloader (mkcert)
- [x] Create first-run detection (`~/.scdev/.initialized`)
- [x] First-run flow: check Docker, install CA, generate certs
- [x] Configure Traefik for HTTPS with generated certs
- [x] Implement `scdev systemcheck`
- [x] Handle sudo for CA installation (with explanation)

**Acceptance Criteria:**
```bash
# First run (fresh system) - triggered by any command or running scdev with no args
./scdev systemcheck
# Welcome message
# Prompts for sudo to install CA (possibly twice - System keychain + Firefox NSS)
# Generates wildcard certificate
# Creates default config

# HTTPS works
curl https://myproject.scalecommerce.site
# Valid certificate, no warnings

# System check (shows status of all dependencies)
./scdev systemcheck
# Docker:        OK (version 24.0.7)
# mkcert:        OK (/usr/local/bin/mkcert v1.4.4)
# Local CA:      OK (trusted)
# Certificates:  OK (*.scalecommerce.site)
```

---

### Milestone 8: Shared Mail (Mailpit) ✅

**Goal:** Catch all project emails in Mailpit

**Tasks:**
- [x] Add Mailpit to shared services
- [x] Connect to project networks (like Traefik)
- [x] Configure Traefik route for `mail.shared.<domain>`
- [x] Implement `scdev mail` (opens browser)

**Acceptance Criteria:**
```bash
# With shared services running
./scdev services status
# Router: running (ports 80, 443)
# Mail:   running (http://mail.shared.scalecommerce.site)

# Access mail UI
./scdev mail
# Opens https://mail.shared.scalecommerce.site in browser

# From project container (with shared.mail: true)
# Use SMTP at mail:1025 (accessible via network alias)
./scdev exec app sh -c 'curl -s smtp://mail:1025'
# Connection succeeds (SMTP port accessible)
```

**Usage in projects:**
Projects with `shared.mail: true` can send email using:
- SMTP Host: `mail`
- SMTP Port: `1025`
- No authentication required

Example environment variables:
- `MAILER_DSN=smtp://mail:1025`
- `SMTP_HOST=mail`
- `SMTP_PORT=1025`

---

### Milestone 9: Justfile Integration ✅

**Goal:** Dynamic commands via Justfiles

**Tasks:**
- [x] Implement tool downloader (just) - with tar.gz extraction support
- [x] Discover `.scdev/commands/*.just` files
- [x] Route unknown commands to justfiles
- [x] Pass environment variables to just (PROJECTNAME, PROJECTPATH, SCDEV_DOMAIN, etc.)
- [x] `scdev <name>` runs `.scdev/commands/<name>.just`
- [x] `scdev <name> --list` shows available tasks
- [x] Handle built-in command priority

**Implementation Notes:**
- Tool download system refactored to support different URL patterns and archive types
- `ToolInfo` struct extended with `ArchiveType` and `URLBuilder` fields
- `JustArch()`/`JustOS()` handle just's naming conventions (x86_64, apple-darwin)
- Command interception in `Execute()` before Cobra's default handling
- Justfiles use `scdev exec` internally for container commands (intentional design)

**Acceptance Criteria:**
```bash
# Given .scdev/commands/setup.just exists
./scdev setup
# Runs default task from setup.just

./scdev setup install
# Runs 'install' task

./scdev setup --list
# Shows available tasks in setup.just

# Unknown command without justfile
./scdev unknown
# Error: Command 'unknown' not found
```

---

### Milestone 10: Shared DB UI (Adminer) ✅

**Goal:** Lightweight database management UI for all project databases

**Tasks:**
- [x] Add Adminer to shared services
- [x] Connect to project networks (like Mail)
- [x] Configure Traefik route for `db.shared.<domain>`
- [x] Implement `scdev db` (opens browser)
- [x] Add docs page with Statiq plugin (catch-all for 404s)
- [x] Implement `scdev services recreate` command

**Acceptance Criteria:**
```bash
# With shared services running
./scdev services status
# Docs:   running (https://docs.shared.scalecommerce.site)
# Router: running (https://router.shared.scalecommerce.site)
# Mail:   running (https://mail.shared.scalecommerce.site)
# DB:     running (https://db.shared.scalecommerce.site)

# Access database UI
./scdev db
# Opens https://db.shared.scalecommerce.site in browser

# Unconfigured URLs redirect to docs page
curl -I https://nonexistent.scalecommerce.site
# HTTP 307 redirect to https://docs.shared.scalecommerce.site/

# Force recreate all shared service containers
./scdev services recreate
# Stops, removes, and recreates all shared containers
```

**Usage in projects:**
Projects with `shared.db: true` can access Adminer to manage their databases.
Adminer connects to databases via the project network using service names as hostnames.

**Database service detection:**
- Auto-detected: services named `db`, `database`, `mysql`, `postgres`, etc. or images containing those names
- Explicit: use `register_to_dbui: true` on any service to add it to Adminer's server list

**Docs Page:**
The docs page is served via the Traefik Statiq plugin (no separate container). It provides:
- Documentation and quick reference
- Links to all shared services
- Catch-all for unconfigured routes (redirects to docs instead of showing 404)

**Services Recreate:**
The `scdev services recreate` command force-rebuilds all shared service containers.
Use this when container configuration changes (new volumes, command args, or labels).
When adding new shared services, update `runServicesRecreate()` in `cmd/services.go`.

---

### Milestone 11: Mutagen File Sync ✅

**Goal:** Fast file synchronization on macOS to replace slow VirtioFS bind mounts

**Background:**
Docker Desktop on macOS uses VirtioFS for file sharing, which can cause performance issues especially with Alpine Linux containers and parallel file operations (like `composer install`). Mutagen provides bidirectional sync between host filesystem and Docker volumes, bypassing VirtioFS entirely.

**Tasks:**
- [x] Add Mutagen tool definition (`internal/tools/mutagen.go`)
- [x] Create Mutagen wrapper (`internal/mutagen/mutagen.go`)
- [x] Add Mutagen config to global and project configs
- [x] Auto-detect macOS and enable by default
- [x] Transform bind mounts to volumes on Start
- [x] Create/resume Mutagen sessions on Start
- [x] Pause sessions on Stop
- [x] Terminate sessions on Down
- [x] Remove sync volumes on `down -v`
- [x] Add CLI commands: `scdev mutagen status/reset/flush`

**Configuration:**

Global config (`~/.scdev/global-config.yaml`):
```yaml
mutagen:
  enabled: auto       # auto = macOS enabled, Linux disabled
  sync_mode: two-way-safe
```

Project config (`.scdev/config.yaml`):
```yaml
mutagen:
  ignore:
    - var/cache
    - var/log
    - "*.log"
```

**Acceptance Criteria:**
```bash
# On macOS, Mutagen is enabled by default
scdev start
# Creates sync volumes, starts sessions, waits for initial sync

# Check sync status
scdev mutagen status
# Shows session status for each bind mount

# Wait for sync to complete
scdev mutagen flush
# Blocks until all changes are synced

# Recreate stuck sessions
scdev mutagen reset
# Terminates and recreates all sessions

# Stop pauses sessions
scdev stop
# Mutagen sessions paused

# Down terminates sessions
scdev down
# Sessions terminated, volumes kept

# Down -v removes sync volumes
scdev down -v
# Sessions terminated, volumes removed
```

**Naming Convention:**
- Session: `scdev-<project>-<service>` (e.g., `scdev-myshop-app`) - Mutagen only allows alphanumeric and hyphens
- Volume: `sync.<service>.<project>.scdev` (Docker allows dots in volume names)

**Built-in ignores:** `.git`, `.DS_Store` (always applied)

---

### Milestone 12: Shared Redis UI (Redis Insights) ✅

**Goal:** Provide a visual Redis browser for all projects using Redis

**Tasks:**
- [x] Add Redis Insights image constant (`internal/config/defaults.go`)
- [x] Add RedisInsightsConfig structs (`internal/config/config.go`)
- [x] Update global config template (`internal/config/templates/global-config.yaml`)
- [x] Create Redis Insights service (`internal/services/redis_insights.go`)
- [x] Add manager methods: Start/Stop/Status/Connect/Disconnect (`internal/services/manager.go`)
- [x] Update CLI commands (`cmd/services.go`)
- [x] Add project integration (`internal/project/project.go`)
- [x] Update documentation (README.md, CLAUDE.md)

**Configuration:**

Project config (`.scdev/config.yaml`):
```yaml
shared:
  redis_insights: true  # Connect to shared Redis Insights
```

**Acceptance Criteria:**
```bash
# Start shared services (includes Redis Insights)
scdev services start
# Redis Insights available at redis.shared.<domain>

# Start project with Redis Insights enabled
scdev start
# Redis Insights connected to project network, can browse redis:6379

# Check status
scdev services status
# Shows Redis: running (https://redis.shared.<domain>)
```

**URL:** `https://redis.shared.<domain>` (port 5540)

---

## Testing Strategy

### Test Categories

| Category | Location | Runs When | Requires |
|----------|----------|-----------|----------|
| Unit | `*_test.go` | `make test` | Nothing |
| Integration | `*_integration_test.go` | `make test-integration` | Docker |

### Unit Test Guidelines

- Test config parsing with various inputs
- Test variable substitution
- Test command building (Docker CLI args)
- Mock the runtime interface for project tests
- Use table-driven tests (Go idiom)

### Integration Test Guidelines

- Use build tag: `//go:build integration`
- Create real containers, networks, volumes
- Clean up after each test
- Use unique names (include test name/timestamp)
- Skip if Docker not available

### Test Fixtures

```
testdata/
└── projects/
    ├── minimal/
    │   └── .scdev/config.yaml      # Just name + one service
    └── full/
        └── .scdev/config.yaml      # Multiple services, volumes, variables
```

Note: Invalid config testing is done via temp directories in unit tests.

---

## Development Workflow

### Per-Milestone Workflow

1. **Plan** - Review milestone tasks, ask questions
2. **Implement** - Build features incrementally
3. **Test** - Write tests, run manually
4. **Review** - Discuss what worked, what didn't
5. **Refine** - Address feedback before next milestone

### Build Commands

```makefile
# Build for current platform
make build

# Run tests
make test

# Run integration tests (requires Docker)
make test-integration

# Build for all platforms
make build-all

# Install locally
make install
```

### Git Workflow

- One commit per logical change
- Commit message format: `milestone-X: description`
- Example: `milestone-0: add cobra CLI structure`

---

## Open Questions for Future Milestones

These don't need answers now, but we'll address them later:

1. **Future:** Observability (OpenObserve) - how much config needed?
2. **Future:** Tunnel (Cloudflare vs frp) - which to implement first?
3. **Templates:** Git-based? Local copies? Registry?
4. **Self-update:** Use which library? Or shell to curl?
5. **Windows:** WSL2 only, or native Windows containers too?

---

## Next Steps

**Milestone 12: Shared Redis UI** is complete.

All core milestones (0-12) are now complete. Additionally implemented:
- `scdev logs` - View container logs with follow and tail options
- `scdev restart` - Stop and start project in one command
- `scdev mail/db/redis/docs` - Open shared service UIs in browser

Future work could include:
- Observability (OpenObserve integration)
- Tunnel support (Cloudflare/frp)
- Project templates (`scdev create <template>`)
- Self-update functionality
- Additional commands: `shell`, `stats` (ctop integration)
