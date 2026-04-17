# scdev - Completo Project Briefing

## Project Summary

scdev is a local development environment framework for web applications, built in Go. It provides single-command startup (`scdev start`), shared infrastructure (Traefik reverse proxy, Mailpit email catcher, Adminer DB UI, Redis Insights), project isolation via Docker networks, cross-project link networks, and project templates (`scdev create`). Target users are web developers on macOS and Linux who run multiple projects locally. Designed to be operable by AI coding agents (Claude Code, Cursor, Copilot) with deterministic, zero-ambiguity environments. Distributed as a single binary with self-update capability via GitHub Releases. An installable skill (`skills/scdev/SKILL.md`) teaches AI agents the full CLI and config format.

## Domain Model

```
GlobalConfig    - singleton at ~/.scdev/global-config.yaml; domain, SSL, Mutagen, shared service images
ProjectConfig   - per-project at .scdev/config.yaml; services, environment, shared flags
State           - singleton at ~/.scdev/state.yaml; registered projects + link networks + routing ports
Service         - a container definition within a project (image, volumes, env, routing)
SharedService   - infrastructure containers (Router, Mail, DB UI, Redis UI) in scdev_shared network
Volume          - named Docker volume, auto-discovered from service volume mounts (no top-level declaration)
Network         - per-project Docker network + shared scdev_shared + per-link scdev_link_<name>
Link            - runtime relationship between projects, stored in global state (not project config)
LinkMember      - a project or project.service attached to a link network
MutagenSession  - file sync session between host directory and Docker volume (macOS)
Justfile        - custom command definition in .scdev/commands/<name>.just
Template        - GitHub repo or local dir with .scdev/ config for `scdev create`
ConfigHash      - sha256 label stamped on every container covering its full expected config
```

**Key relationships:**
- A Project has many Services and optionally MutagenSessions.
- Named volumes are auto-discovered from service `volumes:` entries (if the source doesn't start with `/`, `.`, or `${`, it's a named volume) - no top-level `volumes:` section needed (unlike Docker Compose).
- SharedServices are global singletons connected to project networks on demand.
- Links are runtime relationships stored in global state. Each creates its own Docker network (`scdev_link_<name>`). A Link can have whole projects or individual services as members.
- Across a link, containers reach each other by the long FQDN `<service>.<project>.scdev`, not by project domain (wildcard DNS points at 127.0.0.1).
- State aggregates routing ports across all projects for Traefik config.
- Justfiles extend the CLI with project-specific commands.

## Workflow & Lifecycle

Default project lifecycle: `scdev start` -> develop -> `scdev stop` (or `scdev down` to tear down, `scdev remove` to also delete the project directory registration).

**Start sequence:** check ports -> create network -> create volumes (auto-discovered) -> pull images -> create containers (stamped with `scdev.config-hash` label) -> connect shared services -> connect to member link networks -> start Mutagen sync -> start containers -> register in state.

**Restart vs Update:** `scdev restart` stops and starts the existing containers (no recreate). `scdev update` diffs the current config-hash against what the code would produce now and recreates only services whose config drifted (image, env, volumes, command, working dir, routing labels, ports, aliases).

**Stop:** pause Mutagen -> stop containers -> disconnect shared services.

**Down:** terminate Mutagen -> stop -> remove containers -> remove network -> disconnect from links -> optionally remove volumes (`-v`) -> unregister from state.

**Rename:** stop -> migrate volumes via temp container (Docker has no native volume rename, so we copy data) -> remove old network -> update state + link memberships -> rewrite `name:` in config.yaml (preserving formatting) -> reload and start.

## Key Design Decisions

- **Config-hash drift detection:** Every container (project and shared) gets an `scdev.config-hash` label that is a deterministic sha256 of image, env, volumes, command, working dir, routing labels, ports, and network aliases. `scdev update` and shared-service start compare the stamped hash to the freshly built expected config and recreate on drift. Any new config field that should shape a container MUST flow through `buildContainerConfig` / the shared-service `*ContainerConfig` functions, or drift won't be detected.
- **Named volume auto-discovery:** `parseVolumeMount()` detects named vs bind volumes from service definitions. No separate `volumes:` declaration needed - a deliberate departure from Docker Compose convention.
- **Single shared-service registry:** `AllSharedServices()` in `internal/services/registry.go` is the one place that lists every shared service (name, container, start/stop/status/connect/disconnect, project opt-in flag). Adding a new shared service is a single-file change.
- **Bootstrap helpers:** `withProject(timeout, fn)` and `withDocker(timeout, fn)` in `cmd/shared.go` collapse the timeout + `requireDocker` + `project.Load()` boilerplate every command used to repeat.
- **Atomic state updates:** All mutating state operations (`RegisterProject`, `CreateLink`, `AddLinkMembers`, `RenameProject`) use a `Mutate(fn)` helper that holds the manager's in-process lock for the entire read-modify-write cycle. Cross-process concurrency is NOT protected (two `scdev` invocations at once can still race).
- **Link networks are state, not config:** Links live in `~/.scdev/state.yaml`, not in any project's `config.yaml`. `scdev link create/join/leave/delete` mutates state. On `scdev start`, a project auto-connects to every link it's a member of.
- **Cross-link DNS:** Containers on a link network reach each other by `<service>.<project>.scdev` (FQDN). The short service alias is only injected on the project's own network (since multiple linked projects can have a service called `app`). Always use FQDN + internal port for cross-project calls, not `https://*.scalecommerce.site` (resolves to 127.0.0.1 inside containers).
- **Install layout with symlink:** scdev lives at `~/.scdev/bin/scdev` with a symlink at `/usr/local/bin/scdev`. Enables sudo-less auto-update. Legacy plain-file installs are silently migrated on first run.
- **Background auto-update with banner:** Update check runs at most once per 24h (conditional GET with ETag), downloads the new binary into `~/.scdev/bin/scdev` silently, and the NEXT invocation prints a banner. Opt out via `SCDEV_NO_UPDATE_CHECK=1`.
- **Checksum verification:** Every path that downloads an scdev binary (`install.sh`, `self-update`, background auto-update) fetches `checksums.txt` alongside the binary and verifies SHA256 before marking executable. Releases without `checksums.txt` are rejected, not silently installed.
- **Runtime abstraction:** All Docker operations go through a `Runtime` interface. The `DockerCLI` implementation shells out to the `docker` binary. Exists for future Podman support; do not introduce the Docker SDK.
- **Config variables are NOT env vars:** `variables:` are `${VAR}` placeholders substituted at config-load time (second pass of `LoadProject()`). They do not reach containers - that's what `environment:` is for.
- **Per-service routing domain:** `routing.domain` allows individual services to have custom domains (HTTP/HTTPS only). Useful for frontend + backend splits on the same project.
- **Project templates:** `scdev create` downloads GitHub repos or copies local dirs. Template repos follow naming convention `scdev-template-<name>`. Templates use a `.setup-complete` marker pattern to solve the container startup vs setup circular dependency. Template authoring guide at `templates/README.md`.
- **ui.StatusStep / `scdev step`:** Framework-level progress messages use bold cyan `▶` + two blank leading lines to stand out from the noise of nested command output (apk, composer, npm). Template justfiles should use `@scdev step "..."` instead of `@echo "..."` for top-level phase markers.

## Tech Stack & Architecture

| Component | Technology |
|-----------|-----------|
| Language | Go 1.25 |
| CLI | Cobra |
| Container runtime | Docker CLI (shell out, not SDK - enables future Podman support) |
| File sync | Mutagen (auto-enabled on macOS, disabled on Linux) |
| Task runner | just (justfiles in `.scdev/commands/`) |
| SSL | mkcert (locally-trusted certs) |
| Config format | YAML with `${VAR}` substitution |
| State storage | YAML files |

**Architecture:** Single Go binary, no external dependencies beyond Docker. Packages:
- `cmd/` - Cobra commands (one file per command), bootstrap helpers in `shared.go`
- `internal/config/` - parsing, `defaults.go` (single source of truth for images/versions/domain), two-pass variable substitution
- `internal/runtime/` - Docker abstraction interface + DockerCLI implementation + `confighash.go`
- `internal/project/` - lifecycle (`project.go`), shared service connections, Mutagen sync, link connections, rename
- `internal/services/` - shared infra (router, mail, adminer, redis insights) + `registry.go` (single source of truth)
- `internal/state/` - global registry with `Mutate()` atomic helper
- `internal/mutagen/` - Mutagen binary wrapper
- `internal/create/` - template resolution, validation, local copy, GitHub tarball download
- `internal/tools/` - external tool download (mkcert, just, mutagen)
- `internal/ui/` - terminal output helpers (colors, hyperlinks, `StatusStep`)
- `internal/updatecheck/` - background update checker + auto-installer
- `skills/scdev/SKILL.md` - AI agent integration skill (with `references/` for config examples and template authoring)
- `templates/README.md` - template authoring guide

## Naming Conventions

| Entity | Pattern | Example |
|--------|---------|---------|
| Container | `{service}.{project}.scdev` | `app.myshop.scdev` |
| Network (project) | `{project}.scdev` | `myshop.scdev` |
| Network (link) | `scdev_link_{name}` | `scdev_link_backend` |
| Volume | `{volume}.{project}.scdev` | `db_data.myshop.scdev` |
| Shared container | `scdev_{service}` | `scdev_router` |
| Shared network | `scdev_shared` | - |
| Mutagen session | `scdev-{project}-{service}` | `scdev-myshop-app` |
| Shared service URL | `{service}.shared.{domain}` | `mail.shared.scalecommerce.site` |
| Project URL | `{project}.{domain}` | `myshop.scalecommerce.site` |
| Config hash label | `scdev.config-hash` | stamped on every container |

## CLI Commands

**Lifecycle:** `create <template> [name]`, `start [-q]`, `stop`, `restart`, `update`, `down [-v]`, `remove <name>`, `rename <new-name>`
**Container interaction:** `exec <service> <cmd>`, `logs [-f] [--tail N] [service]`
**Info:** `info`, `status`, `list`, `config`, `open [project]`
**Shared service shortcuts (browser):** `mail`, `db`, `redis`, `docs`
**Shared services (mgmt):** `services start|stop|status|recreate`
**Cross-project networks:** `link create|delete|join|leave|ls|status`
**Mutagen:** `mutagen status|reset|flush`
**System:** `systemcheck [--install-ca]`, `version`, `self-update`, `cleanup [--global]`, `volumes [--global]`
**Internal helpers:** `step <message>` (for template justfiles - bold cyan progress marker)
**Custom:** Any `.scdev/commands/<name>.just` becomes `scdev <name>`

## Shared Services

| Service | Container | URL pattern | Purpose |
|---------|-----------|-------------|---------|
| Router | `scdev_router` | `router.shared.<domain>` | Traefik reverse proxy + SSL termination; 404 catch-all routes to docs page |
| Mail | `scdev_mail` | `mail.shared.<domain>` | Mailpit email catcher (SMTP on port 1025, container name `mail`) |
| DB UI | `scdev_db` | `db.shared.<domain>` | Adminer with auto-detected database servers |
| Redis UI | `scdev_redis` | `redis.shared.<domain>` | Redis Insights browser |
| Docs | via Traefik | `docs.shared.<domain>` | Dynamic docs page, also the 404 catch-all |

Default domain: `scalecommerce.site` (wildcard DNS -> 127.0.0.1 - not a real site, just a resolver trick).

## Configuration

**Two config files:**
1. `~/.scdev/global-config.yaml` - domain, SSL, Mutagen mode, shared service images
2. `.scdev/config.yaml` - per-project services, environment, shared service flags

**Variable substitution** (two-pass to resolve PROJECTNAME):
Built-in: `${PROJECTNAME}`, `${PROJECTPATH}`, `${PROJECTDIR}`, `${SCDEV_HOME}`, `${SCDEV_DOMAIN}`, `${USER}`, `${HOME}`, plus any env var.
User-defined: `variables:` section defines custom `${VAR}` placeholders (resolved in second pass, can reference built-ins; cannot reference each other).

**Key project config flags:**
- `variables: map` - reusable `${VAR}` substitution values (not passed to containers)
- `auto_open_at_start: bool` - open browser after start
- `shared.router|mail|db|redis: bool` - connect to shared services
- `services.<name>.register_to_dbui: bool` - explicitly register in Adminer (auto-detected for db/mysql/postgres names)
- `services.<name>.routing.protocol: http|https|tcp|udp` - routing type
- `services.<name>.routing.domain: string` - custom domain for this service (http/https only)
- `mutagen.ignore: [paths]` - excluded from sync

**No top-level `volumes:` section.** Named volumes auto-discovered from service `volumes:` entries.

## External Tools

Downloaded to `~/.scdev/bin/` on first use: mkcert v1.4.4, just 1.49.0, mutagen 0.18.1. System PATH checked first. Not yet SHA256-verified on download (tracked as a ticket).

## Build & Release

- `make build` / `make build-all` (darwin/linux, arm64/amd64)
- Version injected via ldflags
- GitHub Actions CI builds binaries + `checksums.txt` + creates releases on tag push
- `scdev self-update` and background auto-update both verify SHA256 against the checksums file
- Install: `curl -fsSL https://raw.githubusercontent.com/.../install.sh | sh`

## Testing

- Unit tests: `make test` (mock runtime in `internal/runtime/mock.go`). Run before every commit.
- Integration tests: `make test-integration` (real Docker, `//go:build integration` tag). Run before releases, or when changing project lifecycle / routing / Mutagen / runtime code.
- Test fixtures in `testdata/projects/`.
- Integration tests that tear down shared services MUST snapshot + restore them via `snapshotSharedServices` / `restoreSharedServices` - otherwise they silently break the developer's running environment.

## Terminology

| Term | Meaning |
|------|---------|
| Project | A web application with `.scdev/config.yaml` |
| Service | A container definition within a project config |
| Shared service | Global infrastructure (router, mail, db, redis) |
| Link / link network | A shared Docker network joining selected projects/services across isolation boundaries |
| Named volume | Auto-discovered persistent storage from service volume mounts |
| Justfile | Custom command file in `.scdev/commands/<name>.just` |
| State | Global registry of projects + links at `~/.scdev/state.yaml` |
| Config-hash label | `scdev.config-hash` label stamped on every container for drift detection |
| Domain | Base domain for URL generation (default: `scalecommerce.site`) |
| First-run / systemcheck | Initial setup: install CA, generate certs, verify Docker |
| Skill | AI agent integration file at `skills/scdev/SKILL.md` |
| Template | GitHub repo or local dir for `scdev create` scaffolding |
| .setup-complete | Marker file in templates solving container startup vs setup circular dependency |

## Ticket Conventions

**Common categories:** feature (new CLI command, new shared service, config option), bug fix, refactor, UX improvement (output formatting, error messages), documentation, security hardening, template improvement.

**Acceptance criteria** should be checklists. Include:
- What the command/feature/fix does observably
- Config options or flags if applicable
- Edge cases (what happens on error, missing Docker, project not registered, etc.)
- Whether `make test` / `make test-integration` should pass
- Which docs must stay in sync (`README.md`, `templates/README.md`, `CLAUDE.md`, `CONTRIBUTING.md`, `skills/scdev/SKILL.md`)

**Scope:** One concern per ticket. A new shared service is one ticket. Adding a flag to an existing command is one ticket. Don't combine unrelated changes.

**When proposing internal changes**, reference the correct anchors: `buildContainerConfig()` for container config drift, `AllSharedServices()` registry for shared services, `Mutate()` for state updates, `ContainerNameFor()` for container naming without a loaded Project.

## Example Tickets

### Example: Feature ticket (new CLI command)
**Title:** Add `scdev open` command to launch project URL in the browser
**Description:**
Currently there's no way to re-open a project's URL after `scdev start` - users run `scdev info` and copy-paste. Add `scdev open` (current project) and `scdev open <name>` (registered project).

**Acceptance criteria:**
- [ ] `scdev open` loads the current project and opens `http(s)://<domain>` in the browser
- [ ] `scdev open <name>` looks up the project in state and opens its domain
- [ ] Protocol respects global SSL config (http vs https)
- [ ] Clear error when `<name>` is not registered
- [ ] Tab completion for project names via existing `completeProjectNames` helper
- [ ] README "Information" command block updated
- [ ] SKILL.md CLI reference updated

### Example: Bug fix
**Title:** Fix shared services silently starting stale after config drift
**Description:**
When a shared service was created with one config (e.g., SSL off) and the global config later changed (SSL on, image bump, domain change), `scdev services start` would just start the stale container. Traefik labels, images, and ports wouldn't match the new config.

**Acceptance criteria:**
- [ ] Every shared-service `*ContainerConfig` function stamps `runtime.StampConfigHash`
- [ ] `services.Manager.startService` compares the running container's hash against the expected config and recreates on mismatch
- [ ] Router has a port-superset carve-out (doesn't recreate just because another project shrank its ports)
- [ ] Integration test that flips SSL and asserts shared containers are recreated

### Example: Security hardening ticket
**Title:** Verify SHA256 of downloaded third-party tools (mkcert, just, mutagen)
**Description:**
`internal/tools/tools.go:downloadTool` fetches binaries from GitHub releases and marks them executable with no integrity check. A compromised release or HTTPS MITM via a rogue CA silently runs code on the user's machine. scdev's own binary already verifies SHA256 via `checksums.txt`; the pinned third-party tools still download blind.

**Acceptance criteria:**
- [ ] Add `SHA256 map[string]string` field to `ToolInfo` keyed by `<goos>-<goarch>`
- [ ] Populate hashes for mkcert v1.4.4, just 1.49.0, mutagen 0.18.1
- [ ] `downloadTool` computes sha256 of the downloaded file and compares, hard-fails on mismatch
- [ ] Hard-fail on empty/missing expected hash for the current platform (no silent skip)
- [ ] Document the version-bump workflow (update tool version + hash in the same PR) in CONTRIBUTING.md

### Example: Template ticket
**Title:** Create Django template for `scdev create django`
**Description:**
Add a new project template at `ScaleCommerce-DEV/scdev-template-django` that scaffolds a Django project with PostgreSQL. Use the scaffold-in-place pattern since Django's `startproject` can run in non-empty directories.

**Acceptance criteria:**
- [ ] Template includes `.scdev/config.yaml` with Python 3.12, PostgreSQL, Mutagen ignores
- [ ] `setup.just` installs dependencies and runs `django-admin startproject`, using `@scdev step` for phase headers
- [ ] Container command uses `.setup-complete` marker pattern
- [ ] Dev server binds to `0.0.0.0:8000` for container accessibility
- [ ] `scdev create django my-app && cd my-app && scdev setup` produces a working project
- [ ] README with usage instructions linking to the Template Authoring Guide

## Out of Scope

- Cloud deployment or production use - scdev is strictly for local development
- Windows support - macOS and Linux only
- Docker Compose compatibility - scdev has its own config format
- Multi-machine or remote Docker - local Docker daemon only
- Container image building - scdev uses pre-built images only
- Docker SDK integration - we shell out to the CLI to keep the door open for Podman
