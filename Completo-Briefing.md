# scdev — Completo Project Briefing

## Project Summary

scdev is a local development environment framework for web applications, built in Go. It provides single-command startup (`scdev start`), shared infrastructure (Traefik reverse proxy, Mailpit email catcher, Adminer DB UI, Redis Insights), and project isolation via Docker networks. Target users are web developers on macOS and Linux who run multiple projects locally. Distributed as a single binary with self-update capability via GitHub Releases.

## Domain Model

```
GlobalConfig   — singleton at ~/.scdev/global-config.yaml; domain, SSL, Mutagen, shared service images
ProjectConfig  — per-project at .scdev/config.yaml; services, volumes, environment, shared flags
State          — singleton at ~/.scdev/state.yaml; tracks registered projects + routing ports
Service        — a container definition within a project (image, volumes, env, routing)
SharedService  — infrastructure containers (Router, Mail, DB UI, Redis UI) in scdev_shared network
Volume         — named Docker volume scoped to a project
Network        — per-project Docker network + shared scdev_shared network
MutagenSession — file sync session between host directory and Docker volume (macOS)
Justfile       — custom command definition in .scdev/commands/<name>.just
```

**Key relationships:**
- A Project has many Services, Volumes, and optionally MutagenSessions
- SharedServices are global singletons connected to project networks on demand
- State aggregates routing ports across all projects for Traefik config
- Justfiles extend the CLI with project-specific commands

## Workflow & Lifecycle

Default project lifecycle: `scdev start` → develop → `scdev stop` (or `scdev down` to tear down).

**Start sequence:** check ports → create network → create volumes → pull images → create containers → connect shared services → start Mutagen sync → start containers → register in state.

**Stop:** pause Mutagen → stop containers → disconnect shared services.

**Down:** terminate Mutagen → stop → remove containers → remove network → optionally remove volumes (`-v`) → unregister from state.

## Tech Stack & Architecture

| Component | Technology |
|-----------|-----------|
| Language | Go 1.25 |
| CLI | Cobra + Viper |
| Container runtime | Docker CLI (shell out, not SDK — enables future Podman support) |
| File sync | Mutagen (auto-enabled on macOS, disabled on Linux) |
| Task runner | just (justfiles in `.scdev/commands/`) |
| SSL | mkcert (locally-trusted certs) |
| Config format | YAML with `${VAR}` substitution |
| State storage | YAML files |

**Architecture:** Single Go binary, no external dependencies beyond Docker. Packages: `cmd/` (Cobra commands), `internal/config` (parsing + defaults), `internal/runtime` (Docker abstraction interface), `internal/project` (lifecycle), `internal/services` (shared infra), `internal/mutagen` (sync), `internal/tools` (external tool download), `internal/ui` (terminal output).

**Runtime abstraction:** All Docker operations go through a `Runtime` interface in `internal/runtime/`. The `DockerCLI` implementation shells out to the `docker` binary. This abstraction exists to support future Podman compatibility.

## Naming Conventions

| Entity | Pattern | Example |
|--------|---------|---------|
| Container | `{service}.{project}.scdev` | `app.myshop.scdev` |
| Network | `{project}.scdev` | `myshop.scdev` |
| Volume | `{volume}.{project}.scdev` | `db_data.myshop.scdev` |
| Shared container | `scdev_{service}` | `scdev_router` |
| Shared network | `scdev_shared` | — |
| Mutagen session | `scdev-{project}-{service}` | `scdev-myshop-app` |
| Shared service URL | `{service}.shared.{domain}` | `mail.shared.scalecommerce.site` |
| Project URL | `{project}.{domain}` | `myshop.scalecommerce.site` |

## CLI Commands (25 total)

**Lifecycle:** `start`, `stop`, `restart`, `down [-v]`
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

Default domain: `scalecommerce.site` (wildcard DNS → 127.0.0.1).

## Configuration

**Two config files:**
1. `~/.scdev/global-config.yaml` — domain, SSL, Mutagen defaults, shared service images (auto-created from embedded template)
2. `.scdev/config.yaml` — per-project services, volumes, environment, shared service flags

**Variable substitution** (two-pass to resolve PROJECTNAME):
`${PROJECTNAME}`, `${PROJECTPATH}`, `${PROJECTDIR}`, `${SCDEV_HOME}`, `${SCDEV_DOMAIN}`, `${USER}`, `${HOME}`, plus any env var.

**Key project config flags:**
- `auto_open_at_start: bool` — open browser after start
- `shared.router|mail|db|redis_insights: bool` — connect to shared services
- `services.<name>.register_to_dbui: bool` — explicitly register in Adminer (auto-detected for db/mysql/postgres names)
- `services.<name>.routing.protocol: http|https|tcp|udp` — routing type
- `mutagen.ignore: [paths]` — excluded from sync

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
| Justfile | Custom command file in `.scdev/commands/<name>.just` |
| State | Global registry of projects at `~/.scdev/state.yaml` |
| Domain | Base domain for URL generation (default: `scalecommerce.site`) |
| First-run / systemcheck | Initial setup: install CA, generate certs, verify Docker |

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

### Example: Enhancement
**Title:** Add `--force` flag to `scdev start` for recreating containers
**Description:**
When service configuration changes (new env vars, different image), `scdev start` reuses existing containers. Add a `--force` flag that removes and recreates containers, similar to `docker compose up --force-recreate`.

**Acceptance criteria:**
- [ ] `scdev start --force` removes existing containers before creating new ones
- [ ] Without `--force`, behavior unchanged (reuse existing containers)
- [ ] Mutagen sessions recreated when force-starting
- [ ] Flag documented in `scdev start --help`

## Out of Scope

- Cloud deployment or production use — scdev is strictly for local development
- Windows support — macOS and Linux only
- Docker Compose compatibility — scdev has its own config format
- Multi-machine or remote Docker — local Docker daemon only
- Container image building — scdev uses pre-built images only
