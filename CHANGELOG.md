## v0.6.0

### Features

- **Background auto-update with banner** - scdev checks for updates in the background (at most once per 24h, conditional GET with ETag) and, if the running binary is in the symlink layout, silently downloads the new release into `~/.scdev/bin/scdev`. No sudo, no re-exec. The next invocation prints a one-line banner informing you that you've been upgraded. Legacy installs (plain binary in `/usr/local/bin/`) still get a "run `scdev self-update` to migrate" banner. Opt out with `SCDEV_NO_UPDATE_CHECK=1`; CI environments are detected automatically.
- **Checksum-verified installs and updates** - every path that downloads an scdev binary (`install.sh`, `scdev self-update`, the background auto-update) now fetches `checksums.txt` alongside the binary and verifies SHA256 before chmod-ing it executable. A compromised release asset or HTTPS MITM via a rogue CA can no longer silently run code on your machine. Releases without `checksums.txt` (pre-0.6) are rejected rather than falling back to unchecked install.

### Bug Fixes

- **Fix stale shared-service containers silently breaking after config changes** - if a shared service (Mail, DB UI, Redis Insights, Router) was created while SSL was off and SSL later got flipped on (or image, domain, or dashboard settings changed), `scdev services start` would just start the stale container instead of recreating it, so the new Traefik labels never took effect. Symptoms: `https://db.shared.<domain>` redirecting to the docs page, HTTPS URLs for other shared services falling through to the 404 catch-all. Shared services now carry the same `scdev.config-hash` label that project services use; `startService` compares it to the hash of the freshly built expected config on every call and recreates the container on drift. The router keeps its "don't recreate just to shrink ports" behavior (other projects' ports would be dropped) via a union-of-ports check before hashing.
- **Fix lost updates to the state file** - every mutating operation on the state manager (`RegisterProject`, `CreateLink`, `AddLinkMembers`, `RenameProject`, etc.) used to Load then Save as two independent locked calls, so a concurrent goroutine (like the background update-check doing its own work in parallel with `scdev start`) could interleave and clobber changes. Operations now hold the manager's lock for the entire read-modify-write cycle via a new `Mutate(fn)` helper. Cross-process concurrency (two `scdev` invocations at once on the same state file) is still unprotected - documented, not a regression.
- **Fix `scdev self-update` hanging indefinitely on slow GitHub responses** - the GitHub API call and binary download now use a 5-minute context timeout, matching what the background update-check was already doing. Previously a stalled GitHub response meant the user had to Ctrl-C.

### Improvements

- **Remove unused config fields** - `shared.tunnel` and `shared.observability` were defined in config but never wired to any service manager; enabling them did nothing except (for observability) print an `observe.shared.<domain>` URL in `scdev info` that 404'd. `pre_start` was in `ServiceConfig` and documented in the README but no code ever executed the listed commands. All three are gone. Re-implementation tracked as SC-127, SC-128, SC-146.
- **Remove phantom `persist_on_delete` help text** - `scdev down -v` and `scdev remove -v` flag help advertised "respects persist_on_delete" but no such field exists anywhere in config or code. Removed the claim; if anyone wants the feature for real it's tracked as SC-147.
- **Smaller, safer `cmd/*` bootstrap** - new `withProject(timeout, fn)` / `withDocker(timeout, fn)` helpers in `cmd/shared.go` collapse the timeout + Docker-availability + project-load boilerplate that every `runXxx` repeated. Roughly 150 lines of duplication gone across 15+ commands.
- **Single shared-service registry** - shared service metadata (name, subdomain, container name, start/stop/status/connect/disconnect funcs, per-project opt-in flag) now lives in one place: `services.AllSharedServices()`. Adding a new shared service is a single-file change instead of updating two hand-rolled registries (`cmd/services.go` and `internal/project/shared_services.go`) plus hoping they stay in sync.
- **Other small cleanups** - `strings.Join`/`strconv.Itoa` for the router's port CSV label, `html.EscapeString` replaces a hand-written HTML escaper on the docs page, shared `teardownContainers` helper between `Project.Down` and `Project.Rename`, and cleaner early-return control flow in `NetworkDisconnect`.

## v0.5.6

### Improvements

- **`scdev step` styling refinement** - the whole header line is now bold cyan (not just the `▶`), and a trailing blank line separates the header from the command output that follows. Each phase reads as a framed section, not a single colored glyph next to white text.

## v0.5.5

### Features

- **`scdev step <message>` subcommand** - prints a visually distinct progress marker (two leading blank lines, cyan `▶`, bold text) for use inside `.scdev/commands/*.just` recipes. During `scdev setup`, top-level status messages like "Installing PHP extensions" previously got buried in the wall of apk/composer/npm output; `@scdev step "..."` replaces `@echo` for phase headers so they pop against the noise. Auto-plain on non-TTY, `NO_COLOR`, or global plain mode. `scdev start`'s own "Starting project ..." line now uses the same helper.

### Improvements

- **Template authoring docs updated** - `templates/README.md`, `skills/scdev/references/templates.md`, and the scdev skill now recommend `@scdev step` for setup.just phase headers and explain why, with the base/Nuxt/Symfony examples rewritten to use it.

## v0.5.4

### Bug Fixes

- **Fix `scdev update` ignoring env/image/volume/command changes** - `serviceNeedsRecreate` only compared Traefik labels and bailed out with "Project is up to date" on any other change, leaving stale containers running. It now compares a deterministic `scdev.config-hash` label stamped on each container, covering image, env, volumes, command, working dir, routing, published ports, and network aliases. Containers created before the label existed are recreated once on first update after upgrade.

### Improvements

- **Skill updates for template creation and existing-project setup** - expanded `skills/scdev/` references (new stack-gotchas reference, richer config examples and template authoring notes, updated SKILL.md with rename/link commands and per-service `routing.domain` guidance) to help agents scaffold new templates and onboard scdev into existing projects.
- **README and template docs: Symfony/Sylius/Laravel trusted-proxies troubleshooting** - documented the mixed-content / stuck debug toolbar / broken admin login failure mode caused by Traefik terminating HTTPS without a trusted-proxy env var, plus PHP template authoring tips (memory_limit drop-in, install-php-extensions, asset-pipeline builds).

## v0.5.3

### Bug Fixes

- **Fix systemcheck reporting "All checks passed" when CA is not trusted** - `checkCA` only checked whether CA files exist, not whether the CA is trusted by the system. On a new install where the user skips CA installation, first-run would say "Setup incomplete" but systemcheck would say "All checks passed!" immediately after. Now systemcheck verifies trust status and reports "not trusted by system" with instructions to fix.

## v0.5.2

### Bug Fixes

- **Fix RedisInsights image not resolved on new installs** - `${RedisInsightsImage}` was missing from the substitution map in `generateDefaultGlobalConfig()`, causing the literal string to be written to `~/.scdev/global-config.yaml` on first run. Docker then failed with "invalid reference format". Existing installs were unaffected because their config file predated the redis_insights section.

### Maintenance

- Bump Go toolchain from 1.25.1 to 1.26.2
- Bump just from 1.40.0 to 1.49.0
- Bump golang.org/x/term from v0.39.0 to v0.42.0
- Bump golang.org/x/sys from v0.40.0 to v0.43.0
- Bump github.com/spf13/pflag from v1.0.9 to v1.0.10

## v0.5.1

### Features

- **`scdev link` command** - create named link networks for direct container-to-container communication between separate projects
  - `scdev link create <name>` / `scdev link delete <name>` - manage named link networks
  - `scdev link join <name> <member>...` / `scdev link leave <name> <member>...` - add/remove projects or individual services
  - `scdev link ls` - list all links and their members
  - `scdev link status <name>` - show members and connection state
  - Members can be whole projects (`sec-scan`) or specific services (`redis-debug.app`)
  - Each link creates a dedicated Docker network (`scdev_link_<name>`) for isolation between link groups
  - Containers resolve each other by container name via Docker's embedded DNS (e.g., `app.project-b.scdev`)
  - Links persist in global state and auto-reconnect on `scdev start`
  - Validation: link name characters, project/service existence, duplicate prevention
- **`scdev info` / `scdev status` / `scdev list` show link information** - links are displayed alongside services and shared services

- **`scdev rename` command** - rename a project with full Docker resource migration
  - Stops containers, migrates volume data (named + Mutagen sync), removes old network
  - Updates state file and link memberships atomically
  - Rewrites `name:` in `.scdev/config.yaml` (preserves formatting and comments)
  - Restarts project with new name
  - Confirmation prompt (skip with `--force`)
  - Validates new name is DNS-safe and not already taken

### Bug Fixes

- **Fix variables in typed config fields** - `${VAR}` placeholders in int/bool fields (e.g., `port: ${PORT}`, `router: ${ENABLE}`) caused "cannot unmarshal !!str into int" errors. The first config parse pass now uses a minimal struct, deferring typed field parsing until after variable substitution.

### Improvements

- Added `ContainerNameFor()` standalone helper for building container names without a loaded Project
- Added `CopyVolume()` to Runtime interface for volume data migration

## v0.4.2

### Bug Fixes

- **Fix shared service hostnames not resolving from project containers** - `docker network connect` now passes `--alias` flags, so shared services (e.g., `mail`, `adminer`, `redis-insights`) are resolvable by their short names on project networks, not just on the shared network
- **Fix `Down()` not releasing TCP/UDP host ports** - `Down()` now refreshes the router to drop ports the project was using, preventing port conflicts on subsequent starts (previously only done in CLI commands, not the library method)

### Improvements

- **Integration tests restore shared services** - tests that tear down shared services now snapshot what's running beforehand and restore it after, preventing silent breakage of the developer's running environment
- Updated docs (README, SKILL.md, templates/README.md) with shared service hostname reference - container-internal hostnames and ports for accessing services from app code vs browser

## v0.4.1

### Improvements

- `Down()` now handles state unregistration (previously only done in `cmd/down.go`), fixing stale entries left by integration tests
- Added integration tests for exec `--` separator, config variables in running containers, per-service routing domain, and Down() state cleanup
- Added CONTRIBUTING.md with developer guide, architecture decisions, and test strategy

## v0.4.0

### Features

- **`scdev create` command** - scaffold new projects from templates (GitHub repos or local directories)
  - Template resolution: bare name (`express`), full repo (`myorg/repo`), or local path (`./dir`)
  - `--branch` and `--tag` flags for GitHub templates
  - `--auto-start` and `--auto-setup` flags for non-interactive setup
  - DNS-safe project name validation
  - GitHub tarball download with security hardening (symlink validation, size limits, mode masking, path traversal checks)
- **Config variables** - `variables:` section for reusable `${VAR}` substitution across the config file (not passed to containers). Variables can reference built-in variables like `${PROJECTNAME}`.
- **Per-service routing domain** - `routing.domain` allows individual HTTP/HTTPS services to have custom domains (e.g. `api.my-app.scalecommerce.site` for a backend service)
- **`scdev start -q/--quiet`** - skip project info display after start (useful in scripts and setup.just)
- **Docker availability check** - all Docker-dependent commands now check if Docker is running and show a clear error message instead of confusing Docker errors
- **`scdev exec` handles `--` separator** - `scdev exec app -- cmd` now works correctly

### Templates

- Three official templates published:
  - [express](https://github.com/ScaleCommerce-DEV/scdev-template-express) - Node.js + Express hello world with `--watch` mode
  - [nuxt4](https://github.com/ScaleCommerce-DEV/scdev-template-nuxt4) - Nuxt 4 with interactive scaffolding, HMR, `nuxi prepare` for module deps
  - [symfony](https://github.com/ScaleCommerce-DEV/scdev-template-symfony) - Symfony with CLI dev server, scaffold-in-/tmp pattern
- Template Authoring Guide at `templates/README.md`
- `.setup-complete` marker pattern for solving container startup vs setup circular dependency

### Improvements

- `shared.redis_insights` renamed to `shared.redis` in project config (consistent with `shared.router`, `shared.mail`, `shared.db`)
- `buildContainerConfig` is now the single source of truth for container configuration (fixes divergence between start and update paths)
- `connectRouter` uses shared helper pattern (consistent with mail/db/redis)
- Extracted `IsDBServiceByName()` to eliminate duplicate DB detection logic
- Reduced redundant `GlobalConfig` loading in status command
- Removed dead `sync_mode` code from Mutagen sync
- Fixed unsafe `append` in cleanup command
- Supply chain security messaging in README

### Documentation

- README: Templates section, multi-service routing, configuration reference tables, supply chain security callout
- Template Authoring Guide: setup lifecycle, scaffolding patterns (in-place vs /tmp), framework-specific notes
- CLAUDE.md: templates docs, Docker check, variables, routing.domain
- scdev skill restructured with progressive disclosure (222-line SKILL.md + references/)

## v0.3.0

### Features

- **Sync-ready gate** - containers wait for Mutagen sync automatically, no more `while [ ! -f ... ]` workarounds
- **Default protocol** - `routing.protocol` defaults to `http` when `port` is set
- **Default domain** - `domain` defaults to `{name}.scalecommerce.site` when not set
- **scdev skill** - installable agent skill (`npx skills add scalecommerce-dev/scdev`) with full CLI reference, config templates for Node/PHP/Python, debugging guides, and setup workflows

### Documentation

- Complete README rewrite with architecture diagram, benchmark data, and coding agents section
- New tagline: "Ever seen a developer and an AI agent fall in love with a dev environment?"
- File sync benchmark: 5x faster cold start vs Docker bind mounts on macOS
- "Standing on the Shoulders of Giants" section crediting all underlying technologies
- TCP/UDP routing, volumes, and custom commands documented in detail
- `.pnpm-store` must be in Mutagen ignore for pnpm projects (prevents native binary corruption across image changes)

## v0.2.0

### Improvements

- Shared service registry pattern — adding a new shared service is now one entry instead of editing 6+ locations
- Split `project.go` god file (1400→868 lines) into `shared_services.go` and `mutagen_sync.go`
- Single source of truth for global config defaults (`newDefaultGlobalConfig()`) — fixes RedisInsights image missing bug
- Removed unused `StartRouterWithPorts` parameters and unnecessary `ConnectToProject`/`DisconnectFromProject` aliases
- Fixed `scdev info` not showing shared services section when only `redis_insights` is enabled
- Consistent display names across all shared service methods

## v0.1.1

### Improvements

- Auto-discover named volumes from service definitions — no more redundant top-level `volumes:` section in project config
- Streamlined CLAUDE.md and planning docs — removed stale TODOs, marked completed phases
- Removed TODO comments from code — all tracked as Completo tickets

## v0.1.0

### Initial Release

First public release of scdev - local development environment framework.

- Single-command project startup with `scdev start`
- Config-driven multi-service containers via `.scdev/config.yaml`
- Variable substitution (`${PROJECTNAME}`, `${PROJECTDIR}`, `${SCDEV_DOMAIN}`)
- Project isolation via Docker networks with inter-service DNS
- Named volumes with persistence across restarts
- Project state tracking with `scdev list` and `scdev info`
- Shared Traefik router with automatic HTTPS via mkcert
- Shared Mailpit email catcher
- Shared Adminer database UI
- Shared Redis Insights browser
- Custom project commands via justfiles (`.scdev/commands/`)
- Mutagen file sync for fast macOS development
- Docs page with dynamic project list at `docs.shared.<domain>`
- Self-update command (`scdev update`)
- Install script for macOS and Linux
