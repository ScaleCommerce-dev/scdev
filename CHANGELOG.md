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
