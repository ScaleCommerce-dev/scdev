# scdev - Completo Project Briefing

## Project Summary

scdev is a local development environment framework for web applications, built in Go. It provides single-command startup (`scdev start`), shared infrastructure (Traefik reverse proxy, Mailpit email catcher, Adminer DB UI, Redis Insights), project isolation via Docker networks, and project templates (`scdev create`). Target users are web developers on macOS and Linux who run multiple projects locally. All code runs inside containers, protecting host machines from supply chain attacks. Designed to be operable by AI coding agents (Claude Code, Cursor, Copilot) with deterministic, zero-ambiguity environments. Distributed as a single binary with self-update capability via GitHub Releases. An installable skill (`npx skills add scalecommerce-dev/scdev`) teaches AI agents the full CLI and config format.

## Domain Model

```
GlobalConfig   - singleton at ~/.scdev/global-config.yaml; domain, SSL, Mutagen, shared service images
                 Defaults extracted to newDefaultGlobalConfig() - single source of truth
ProjectConfig  - per-project at .scdev/config.yaml; services, environment, shared flags
State          - singleton at ~/.scdev/state.yaml; tracks registered projects + routing ports
Service        - a container definition within a project (image, volumes, env, routing)
SharedService  - infrastructure containers (Router, Mail, DB UI, Redis UI) in scdev_shared network
                 Managed via registry pattern in both cmd/services.go and project/shared_services.go
Volume         - named Docker volume, auto-discovered from service volume mounts (no top-level declaration)
Network        - per-project Docker network + shared scdev_shared network
MutagenSession - file sync session between host directory and Docker volume (macOS)
Justfile       - custom command definition in .scdev/commands/<name>.just
Template       - GitHub repo or local dir with .scdev/ config for `scdev create`
TemplateSource - resolved template location (local path or GitHub owner/repo/ref)
```

**Key relationships:**
- A Project has many Services and optionally MutagenSessions
- Named volumes are auto-discovered from service `volumes:` entries (if path doesn't start with `/` or `.`, it's a named volume) - no top-level `volumes:` section needed (unlike Docker Compose)
- SharedServices are global singletons connected to project networks on demand
- State aggregates routing ports across all projects for Traefik config
- Justfiles extend the CLI with project-specific commands

## Workflow & Lifecycle

Default project lifecycle: `scdev start` -> develop -> `scdev stop` (or `scdev down` to tear down).

**Start sequence:** check ports -> create network -> create volumes (auto-discovered) -> pull images -> create containers -> connect shared services (via registry) -> start Mutagen sync -> start containers -> register in state.

**Stop:** pause Mutagen -> stop containers -> disconnect shared services.

**Down:** terminate Mutagen -> stop -> remove containers -> remove network -> optionally remove volumes (`-v`) -> unregister from state.

## Key Design Decisions

- **Named volume auto-discovery:** `parseVolumeMount()` detects named vs bind volumes from service definitions. No separate `volumes:` declaration needed. This is a deliberate departure from Docker Compose convention.
- **Shared service registry pattern:** Both `cmd/services.go` and `internal/project/shared_services.go` use registry arrays. Adding a new shared service is one entry per registry instead of editing multiple switch statements and functions.
- **project.go split:** The monolithic `project.go` (1400 lines) was split into `project.go` (868 lines, core lifecycle), `shared_services.go` (connect/disconnect logic), and `mutagen_sync.go` (file sync lifecycle).
- **GlobalConfig defaults:** `newDefaultGlobalConfig()` in `loader.go` is the single source of truth for default values, eliminating duplication that previously caused bugs (e.g., missing RedisInsights image).
- **StartRouter() simplified:** `StartRouterWithPorts` was collapsed into `StartRouter()` - unused port parameters removed.
- **ConnectToProject/DisconnectFromProject removed:** These were unnecessary aliases that added indirection without value.
- **AI agent compatibility:** scdev is designed to be operable by coding agents. Predictable URLs, single config file, discoverable commands via `ls .scdev/commands/`.
- **Runtime abstraction:** All Docker operations go through a `Runtime` interface (`internal/runtime/`). The `DockerCLI` implementation shells out to the `docker` binary. Exists for future Podman support.
- **Docker availability check:** All Docker-dependent commands call `requireDocker(ctx)` first, providing a clear error if Docker isn't running.
- **Config variables:** `variables:` section in project config defines reusable `${VAR}` placeholders. Substituted in the second pass of `LoadProject()` after PROJECTNAME is resolved. Not passed to containers (that's `environment:`).
- **Per-service routing domain:** `routing.domain` allows individual services to have custom domains (HTTP/HTTPS only). Enables frontend + backend on separate subdomains within the same project.
- **Project templates:** `scdev create` downloads GitHub repos or copies local dirs. Template repos follow naming convention `scdev-template-<name>`. Templates use a `.setup-complete` marker pattern to solve the container startup vs setup circular dependency. Template authoring guide at `templates/README.md`.

## Tech Stack & Architecture

| Component | Technology |
|-----------|-----------|
| Language | Go 1.25 |
| CLI | Cobra + Viper |
| Container runtime | Docker CLI (shell out, not SDK - enables future Podman support) |
| File sync | Mutagen (auto-enabled on macOS, disabled on Linux) |
| Task runner | just (justfiles in `.scdev/commands/`) |
| SSL | mkcert (locally-trusted certs) |
| Config format | YAML with `${VAR}` substitution |
| State storage | YAML files |

**Architecture:** Single Go binary, no external dependencies beyond Docker. Packages:
- `cmd/` - Cobra commands, shared service registry (`sharedServiceDef`)
- `internal/config/` - parsing, defaults (`defaults.go`), variable substitution, `newDefaultGlobalConfig()`
- `internal/runtime/` - Docker abstraction interface + DockerCLI implementation
- `internal/project/` - lifecycle (`project.go`), shared service connections (`shared_services.go`), Mutagen sync (`mutagen_sync.go`)
- `internal/services/` - shared infra (router, mail, adminer, redis insights)
- `internal/mutagen/` - Mutagen binary wrapper
- `internal/create/` - template resolution, validation, local copy, GitHub tarball download
- `internal/tools/` - external tool download (mkcert, just, ctop, mutagen)
- `internal/ui/` - terminal output helpers
- `skills/scdev/SKILL.md` - AI agent integration skill (with `references/` for config examples and template authoring)
- `templates/README.md` - template authoring guide

## Naming Conventions

| Entity | Pattern | Example |
|--------|---------|---------|
| Container | `{service}.{project}.scdev` | `app.myshop.scdev` |
| Network | `{project}.scdev` | `myshop.scdev` |
| Volume | `{volume}.{project}.scdev` | `db_data.myshop.scdev` |
| Shared container | `scdev_{service}` | `scdev_router` |
| Shared network | `scdev_shared` | - |
| Mutagen session | `scdev-{project}-{service}` | `scdev-myshop-app` |
| Shared service URL | `{service}.shared.{domain}` | `mail.shared.scalecommerce.site` |
| Project URL | `{project}.{domain}` | `myshop.scalecommerce.site` |

## CLI Commands

**Lifecycle:** `create <template> [name]`, `start [-q]`, `stop`, `restart`, `down [-v]`
**Container interaction:** `exec [service] [cmd]`, `logs [-f] [--tail N] [service]`
**Info:** `info [--raw]`, `status`, `list`, `config`
**Shared services:** `services start|stop|status|recreate`, `mail`, `db`, `redis`, `docs`
**Mutagen:** `mutagen status|reset|flush`
**System:** `systemcheck [--install-ca]`, `version`, `self-update`, `cleanup [--global]`, `volumes [--global]`
**Custom:** Any `.scdev/commands/<name>.just` becomes `scdev <name>`

## Shared Services

| Service | Container | URL pattern | Purpose |
|---------|-----------|-------------|---------|
| Router | `scdev_router` | `router.shared.<domain>` | Traefik reverse proxy + SSL termination |
| Mail | `scdev_mail` | `mail.shared.<domain>` | Mailpit email catcher (SMTP on port 1025) |
| DB UI | `scdev_db` | `db.shared.<domain>` | Adminer with auto-detected database servers |
| Redis UI | `scdev_redis` | `redis.shared.<domain>` | Redis Insights browser |
| Docs | via Traefik | `docs.shared.<domain>` | Dynamic docs page, 404 catch-all |

Default domain: `scalecommerce.site` (wildcard DNS -> 127.0.0.1).

## Configuration

**Two config files:**
1. `~/.scdev/global-config.yaml` - domain, SSL, Mutagen defaults, shared service images (auto-created from embedded template, defaults via `newDefaultGlobalConfig()`)
2. `.scdev/config.yaml` - per-project services, environment, shared service flags

**Variable substitution** (two-pass to resolve PROJECTNAME):
Built-in: `${PROJECTNAME}`, `${PROJECTPATH}`, `${PROJECTDIR}`, `${SCDEV_HOME}`, `${SCDEV_DOMAIN}`, `${USER}`, `${HOME}`, plus any env var.
User-defined: `variables:` section defines custom `${VAR}` placeholders (resolved in second pass, can reference built-ins).

**Key project config flags:**
- `variables: map` - reusable `${VAR}` substitution values (not passed to containers)
- `auto_open_at_start: bool` - open browser after start
- `shared.router|mail|db|redis: bool` - connect to shared services
- `services.<name>.register_to_dbui: bool` - explicitly register in Adminer (auto-detected for db/mysql/postgres names)
- `services.<name>.routing.protocol: http|https|tcp|udp` - routing type
- `services.<name>.routing.domain: string` - custom domain for this service (http/https only)
- `mutagen.ignore: [paths]` - excluded from sync

**No top-level `volumes:` section.** Named volumes are auto-discovered from service `volumes:` entries. If a volume source doesn't start with `/` or `.`, it's treated as a named volume.

## External Tools

Downloaded to `~/.scdev/bin/` on first use: mkcert v1.4.4, just v1.40.0, ctop v0.7.7, mutagen v0.18.1. System PATH checked first.

## Build & Release

- `make build` / `make build-all` (darwin/linux, arm64/amd64)
- Version injected via ldflags
- GitHub Actions CI builds binaries + creates releases on tag push
- `scdev self-update` checks GitHub Releases
- Install: `curl -fsSL https://raw.githubusercontent.com/.../install.sh | sh`

## Testing

- Unit tests: `make test` (mock runtime in `internal/runtime/mock.go`)
- Integration tests: `make test-integration` (requires Docker, `//go:build integration` tag)
- Test fixtures in `testdata/projects/`

## Terminology

| Term | Meaning |
|------|---------|
| Project | A web application with `.scdev/config.yaml` |
| Service | A container definition within a project config |
| Shared service | Global infrastructure (router, mail, db, redis) |
| Named volume | Auto-discovered persistent storage from service volume mounts |
| Justfile | Custom command file in `.scdev/commands/<name>.just` |
| State | Global registry of projects at `~/.scdev/state.yaml` |
| Domain | Base domain for URL generation (default: `scalecommerce.site`) |
| First-run / systemcheck | Initial setup: install CA, generate certs, verify Docker |
| Registry pattern | Array of service definitions replacing scattered switch statements |
| Skill | AI agent integration file at `skills/scdev/SKILL.md` |
| Template | GitHub repo or local dir for `scdev create` scaffolding |
| .setup-complete | Marker file in templates solving container startup vs setup circular dependency |

## Ticket Conventions

**Common categories:** feature (new CLI command, new shared service, config option), bug fix, refactor, UX improvement (output formatting, error messages), documentation.

**Acceptance criteria** should be checklists. Include:
- What the command/feature does
- Config options if applicable
- Edge cases (what happens on error, missing Docker, etc.)
- Whether `make test` or `make test-integration` should pass
- Whether CLAUDE.md needs updating

**Scope:** Tickets should be scoped to a single concern. A new shared service is one ticket. Adding a flag to an existing command is one ticket. Don't combine unrelated changes.

## Example Tickets

### Example: Feature ticket
**Title:** Add `scdev top` command for container monitoring
**Description:**
Add a `top` command that launches ctop filtered to the current project's containers. The ctop binary is already managed by the tools package.

**Acceptance criteria:**
- [ ] `scdev top` launches ctop with filter for `*.{project}.scdev` containers
- [ ] Works without a project context (shows all scdev containers)
- [ ] Downloads ctop on first use if not installed
- [ ] Unit test for command registration

### Example: Bug fix
**Title:** Fix `scdev logs` returning empty output
**Description:**
`scdev logs` returns nothing because the command gets double-wrapped in `sh -c`. The shell command passed to `docker logs` needs to be the container name directly, not wrapped.

**Acceptance criteria:**
- [ ] `scdev logs app` shows container output
- [ ] `scdev logs -f app` streams logs in real-time
- [ ] `scdev logs --tail 50 app` limits output correctly

### Example: Refactor ticket
**Title:** Extract shared service registry in cmd/services.go
**Description:**
Replace scattered switch statements with a `sharedServiceDef` registry array. Adding a new shared service should require adding one entry to the registry, not editing multiple functions.

**Acceptance criteria:**
- [ ] `sharedServiceRegistry()` returns ordered list of service definitions
- [ ] `services start/stop/status/recreate` all iterate the registry
- [ ] No behavioral changes - all existing commands work identically
- [ ] `make test` passes

### Example: Template ticket
**Title:** Create Django template for `scdev create django`
**Description:**
Add a new project template at `ScaleCommerce-DEV/scdev-template-django` that scaffolds a Django project with PostgreSQL. Use the scaffold-in-place pattern since Django's `startproject` can run in non-empty directories.

**Acceptance criteria:**
- [ ] Template includes `.scdev/config.yaml` with Python 3.12, PostgreSQL, Mutagen ignores
- [ ] `setup.just` installs dependencies and runs `django-admin startproject`
- [ ] Container command uses `.setup-complete` marker pattern
- [ ] Dev server binds to `0.0.0.0:8000` for container accessibility
- [ ] `scdev create django my-app && cd my-app && scdev setup` produces a working project
- [ ] README with usage instructions linking to Template Authoring Guide

## Out of Scope

- Cloud deployment or production use - scdev is strictly for local development
- Windows support - macOS and Linux only
- Docker Compose compatibility - scdev has its own config format
- Multi-machine or remote Docker - local Docker daemon only
- Container image building - scdev uses pre-built images only
