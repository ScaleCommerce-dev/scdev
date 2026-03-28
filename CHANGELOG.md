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
