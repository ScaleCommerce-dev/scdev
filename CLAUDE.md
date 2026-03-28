# scdev

Local development environment framework for web applications. Single command startup, shared infrastructure (Traefik, Mailpit, Adminer), project isolation via Docker networks.

## Current Status

**Milestone 12: Shared Redis UI** - Complete. All core milestones done.

## Quick Reference

```bash
make build           # Build binary
make build-all       # Cross-compile for all platforms
make test            # Run unit tests
make test-integration # Run integration tests (requires Docker)
./scdev version      # Test the binary
```

## Releases

Releases are automated via GitHub Actions (`.github/workflows/release.yml`).

**Release process:**
1. Update `CHANGELOG.md` — add new `## vX.Y.Z` section at top
2. Commit, tag, push:
   ```bash
   git add CHANGELOG.md
   git commit -m "Release vX.Y.Z"
   git tag vX.Y.Z
   git push origin main && git push origin vX.Y.Z
   ```
3. CI builds binaries for darwin/linux (arm64/amd64), creates GitHub Release with changelog

**Version injection:** ldflags set `cmd.Version` and `cmd.BuildTime` at build time.

**Self-update:** `scdev self-update` checks GitHub Releases and replaces the binary.

**Install:** `curl -fsSL https://raw.githubusercontent.com/ScaleCommerce-DEV/scdev/main/install.sh | sh`

## Global Flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Path to project directory containing `.scdev/` (overrides auto-discovery) |

**Use case:** Bootstrap projects with tools that require empty directories (e.g., `composer create-project`):

```bash
# 1. Create config in separate location
mkdir -p /tmp/shopware-bootstrap/.scdev
cat > /tmp/shopware-bootstrap/.scdev/config.yaml << 'EOF'
name: shopware
services:
  app:
    image: php:8.2-cli
    volumes:
      - /path/to/empty/shopware:/app  # Use absolute path
    working_dir: /app
EOF

# 2. Start containers with external config
scdev --config /tmp/shopware-bootstrap start

# 3. Run composer in the container
scdev --config /tmp/shopware-bootstrap exec app composer create-project shopware/production .

# 4. Move config into project (optional)
mv /tmp/shopware-bootstrap/.scdev /path/to/empty/shopware/
```

## Project Structure

```
cmd/                    # Cobra CLI commands
internal/
  config/               # Config parsing, variable substitution
    defaults.go         # All default values (domain, images, tool versions)
    templates/          # Embedded templates with ${VAR} substitution
  mutagen/              # Mutagen file sync wrapper
  runtime/              # Container runtime abstraction (Docker CLI)
  project/              # Project lifecycle, state management
  services/             # Shared services (Traefik, Mailpit, Adminer)
  tools/                # External tool management (just, mkcert, mutagen)
  ui/                   # Terminal output helpers
testdata/projects/      # Test fixtures
planning/               # Design docs and implementation plan
```

## Key Decisions

- **Runtime:** Shell out to `docker` CLI (not SDK) - enables future Podman support
- **Global Config:** `~/.scdev/global-config.yaml` - auto-created from template (distinct from project config.yaml)
- **State:** `~/.scdev/state.yaml` - tracks registered projects
- **Defaults:** All in `internal/config/defaults.go` - single source of truth for domain, images, versions
- **Templates:** Embedded via `//go:embed` with `${VAR}` substitution - easy to maintain
- **CLI:** Cobra + Viper
- **Tests:** Unit tests from start, integration tests tagged `//go:build integration`
- **Default Domain:** `scalecommerce.site` - wildcard DNS resolving to 127.0.0.1

## Milestones

| # | Name | Status |
|---|------|--------|
| 0 | Project Skeleton | Done |
| 1 | Single Container | Done |
| 2 | Config-Driven | Done |
| 3 | Networking | Done |
| 4 | Volumes | Done |
| 5 | Project State | Done |
| 6 | Shared Router | Done |
| 7 | SSL & First-Run | Done |
| 8 | Shared Mail | Done |
| 9 | Justfile | Done |
| 10 | Shared DB UI | Done |
| 11 | Mutagen File Sync | Done |
| 12 | Shared Redis UI | Done |

## Shared Services

Shared services run in the `scdev_shared` network and are managed by `internal/services/manager.go`.

| Service | Container | URL | Purpose |
|---------|-----------|-----|---------|
| Docs | (via Traefik Statiq plugin) | `docs.shared.<domain>` | Documentation page, 404 catch-all |
| Router | `scdev_router` | `router.shared.<domain>` | Traefik reverse proxy |
| Mail | `scdev_mail` | `mail.shared.<domain>` | Mailpit email catcher |
| DB UI | `scdev_db` | `db.shared.<domain>` | Adminer database manager |
| Redis UI | `scdev_redis` | `redis.shared.<domain>` | Redis Insights browser |

### Adding New Shared Services

When adding a new shared service:

1. Add container name constant in `internal/services/<service>.go`
2. Add `Start<Service>`, `Stop<Service>`, `<Service>Status` methods to `manager.go`
3. **Update `cmd/services.go` `runServicesRecreate()`** - add stop/remove/start calls
4. Update `cmd/services.go` start/stop/status commands
5. Add image constant to `internal/config/defaults.go`
6. Update `internal/config/config.go` with config struct if needed

The `scdev services recreate` command force-rebuilds all containers - essential when container config changes (new volumes, args, labels).

## Docs Page

The docs page (`docs.shared.<domain>`) is served via Traefik's Statiq plugin and includes:
- Links to all shared services
- **Dynamic projects list** with running/stopped status (updated on every `scdev start/stop/down`)
- Quick start guide and command reference

Unmatched URLs redirect to docs (catch-all route) instead of showing ugly 404.

## Project Config Options

Key project config options in `.scdev/config.yaml`:

| Option | Type | Description |
|--------|------|-------------|
| `auto_open_at_start` | bool | Open project URL in browser after `scdev start` |
| `shared.router` | bool | Connect to shared Traefik router |
| `shared.mail` | bool | Connect to shared Mailpit |
| `shared.db` | bool | Connect to shared Adminer |
| `shared.redis_insights` | bool | Connect to shared Redis Insights |

### Service Config Options

| Option | Type | Description |
|--------|------|-------------|
| `register_to_dbui` | bool | Explicitly register service in Adminer (auto-detected for services named `db`, `mysql`, `postgres`, or images containing those) |

## Justfile Integration

Projects can define custom commands via justfiles in `.scdev/commands/`:

```
.scdev/
  commands/
    setup.just    # scdev setup
    test.just     # scdev test
    build.just    # scdev build
```

**Usage:**
```bash
scdev setup              # Run default recipe in setup.just
scdev setup install      # Run 'install' recipe
scdev setup --list       # List available recipes
```

**Environment variables passed to just:**
- `PROJECTNAME` - Project name from config
- `PROJECTPATH` - Absolute path to project
- `PROJECTDIR` - Directory basename
- `SCDEV_DOMAIN` - Base domain (e.g., scalecommerce.site)
- `SCDEV_HOME` - ~/.scdev path
- All project `environment:` vars from config

**Container commands in justfiles:**
Use `scdev exec` to run commands inside containers:
```just
install:
    scdev exec app npm ci

test:
    scdev exec app npm test
```

**Command resolution:**
1. Check if built-in command (start, stop, exec, etc.)
2. Check if `.scdev/commands/<name>.just` exists
3. Fall back to Cobra's "unknown command" error

## Mutagen File Sync

Mutagen provides fast bidirectional file sync between host filesystem and Docker volumes, solving VirtioFS performance issues on macOS.

**Auto-detection:**
- macOS: Mutagen enabled by default (bind mounts are slow)
- Linux: Mutagen disabled by default (native bind mounts are fast)

**Global config** (`~/.scdev/global-config.yaml`):
```yaml
mutagen:
  enabled: auto       # auto, true, or false
  sync_mode: two-way-safe
```

**Project config** (`.scdev/config.yaml`):
```yaml
mutagen:
  ignore:
    - var/cache
    - var/log
    - "*.log"
```

**How it works:**
1. On `scdev start`: Creates Docker volumes, starts containers with volumes instead of bind mounts, creates Mutagen sync sessions
2. On `scdev stop`: Pauses sync sessions
3. On `scdev down`: Terminates sync sessions, optionally removes sync volumes with `-v`

**CLI commands:**
- `scdev mutagen status` - Show sync status for project
- `scdev mutagen reset` - Recreate sync sessions (if stuck)
- `scdev mutagen flush` - Wait for sync completion

**Naming convention:** `scdev-<project>-<service>` (e.g., `scdev-myshop-app`) - Mutagen only allows alphanumeric and hyphens

**Note:** Only directory bind mounts are synced via Mutagen. Single-file mounts remain as regular bind mounts.

**Note:** Ignored paths are NOT synced in either direction. This affects IDE autocomplete if `vendor/` or `node_modules/` are ignored.

## Additional Commands

### Logs
```bash
scdev logs [service]     # View logs (defaults to first service)
scdev logs -f app        # Follow logs in real-time
scdev logs --tail 50 app # Show last 50 lines
```

### Restart
```bash
scdev restart            # Stop + start the project
```

### Open Shared Service UIs
```bash
scdev mail               # Open Mailpit in browser
scdev db                 # Open Adminer in browser
scdev redis              # Open Redis Insights in browser
scdev docs               # Open docs page in browser
```

## Detailed Docs

- `planning/implementation-plan.md` - Full milestone details, acceptance criteria
- `planning/scdev-project-briefing.md` - Original design doc (reference when needed)
