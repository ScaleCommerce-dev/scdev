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
