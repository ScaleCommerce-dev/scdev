---
name: scdev
description: |
  Local development environment framework using Docker. Use this skill whenever the user's project
  has a `.scdev/` directory, when they ask to set up a local dev environment, start/stop containers,
  run commands inside containers, check logs, configure routing, or debug container issues.
  Also trigger when: the user says "start the project", "run tests in docker", "check the logs",
  "open mailpit", "set up scdev", "add scdev to this project", mentions `.scdev/config.yaml`,
  or asks about shared services, HTTPS certificates, or file sync on macOS.
  Also trigger when the user wants to create an scdev template, scaffold a new project template,
  build a starter template for a framework, or asks about `scdev create`, template authoring,
  setup.just files, or the .setup-complete pattern.
---

# scdev - Local Development Environment

scdev manages Docker-based local dev environments. One command starts an entire project with HTTPS,
routing, shared services (mail, DB browser, Redis browser), and container isolation.

**How it works:** Each project has `.scdev/config.yaml` defining its services. `scdev start` creates
an isolated Docker network, starts containers, and connects shared services. Traefik routes HTTPS
to `https://{name}.scalecommerce.site` (wildcard DNS to 127.0.0.1). On macOS, Mutagen provides
fast file sync.

## Prerequisites

```bash
scdev version  # Check if installed
# If not: curl -fsSL https://raw.githubusercontent.com/ScaleCommerce-DEV/scdev/main/install.sh | sh && scdev systemcheck
```

## CLI Reference

```bash
# Project lifecycle
scdev create <template> [name]   # Create from template (GitHub repo or local dir)
scdev start                      # Start project
scdev start -q                   # Start quietly (no info display, for scripts)
scdev stop                       # Stop containers
scdev restart                    # Stop + start
scdev down                       # Remove containers and network
scdev down -v                    # Remove everything including volumes

# Run commands in containers
scdev exec <service> <command>   # e.g. scdev exec app pnpm test
scdev exec app bash              # Interactive shell

# Logs and info
scdev logs -f app                # Follow logs
scdev info / status / config     # Project info, status, resolved config

# Shared services
scdev mail / db / redis          # Open in browser
scdev services status            # Check shared services
scdev services recreate          # Rebuild shared containers

# Templates
scdev create express my-app              # ScaleCommerce-DEV/scdev-template-express
scdev create nuxt4 my-app               # ScaleCommerce-DEV/scdev-template-nuxt4
scdev create myorg/my-template my-app    # Any GitHub repo
scdev create ./local-dir my-app          # Local directory
scdev create express my-app --branch dev # Specific branch/tag
```

After `scdev create`: `cd my-app && scdev setup`

## Project Configuration

### Minimal config

```yaml
name: my-app

services:
  app:
    image: node:22-alpine
    command: corepack enable && pnpm install && pnpm dev --host 0.0.0.0
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app
    routing:
      port: 3000

mutagen:
  ignore:
    - node_modules
    - .pnpm-store
```

Result: `https://my-app.scalecommerce.site` with HTTPS, isolation, shared services.

### Shared services: hostnames and access

Shared services have two access modes: **web UI** (browser) and **container-internal** (from app code).

| Service | Web UI URL | Container hostname | Container port | Purpose |
|---------|-----------|-------------------|----------------|---------|
| Mailpit | `mail.shared.scalecommerce.site` | `mail` | `1025` (SMTP) | Email catching |
| Adminer | `db.shared.scalecommerce.site` | `adminer` | `8080` (HTTP) | Database browser |
| Redis Insights | `redis.shared.scalecommerce.site` | `redis-insights` | `5540` (HTTP) | Redis browser |
| Traefik | `router.shared.scalecommerce.site` | `router` | - | Routing dashboard |

**From app containers** (e.g., configuring mail in your app):
- SMTP: `mail:1025` (no auth, no encryption) - all outgoing mail is caught by Mailpit
- Database: use the project service name (e.g., `db:5432` for postgres), NOT `adminer`
- Redis: use the project service name (e.g., `redis:6379`), NOT `redis-insights`

**From browser** (web UIs):
- URLs follow the pattern `https://{service}.shared.scalecommerce.site` (or `http://` without TLS)
- If a shared service isn't running, the URL redirects to the docs page (this is normal)
- `scdev mail`, `scdev db`, `scdev redis` open the web UIs directly

**Important distinction:** Adminer and Redis Insights are **browsers** - they connect to your
project's own database/Redis services. They don't provide a database or Redis themselves. Your app
connects to its own `db` or `redis` service, not to `adminer` or `redis-insights`.

### Full config

```yaml
name: my-project

variables:              # reusable ${VAR} substitution (NOT passed to containers)
  DB_PASSWORD: postgres
  DB_NAME: ${PROJECTNAME}

shared:
  router: true          # Traefik (default: true)
  mail: true            # Mailpit
  db: true              # Adminer
  redis: true            # Redis Insights

environment:            # env vars for ALL containers
  APP_ENV: dev

services:
  app:
    image: node:22-alpine
    command: corepack enable && pnpm install && pnpm dev --host 0.0.0.0
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app        # bind mount (source code)
    environment:                   # env vars for THIS container only
      DATABASE_URL: postgres://postgres:${DB_PASSWORD}@db:5432/${DB_NAME}
    routing:
      protocol: http               # http (default), https, tcp, udp
      port: 3000

  db:
    image: postgres:16-alpine
    volumes:
      - db_data:/var/lib/postgresql/data  # named volume (persistent)
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: ${DB_NAME}

mutagen:
  ignore:
    - node_modules
    - .pnpm-store
    - .nuxt
    - .output
```

### Key concepts

**Variables** (`variables:`) are substituted as `${VAR}` throughout the config file. NOT passed to
containers. Use for values shared across services (DB passwords, names).

**Environment** - project-level `environment:` goes to ALL containers. `services.<name>.environment`
goes to THAT container only and overrides project-level.

**Built-in variables:** `${PROJECTPATH}`, `${PROJECTNAME}`, `${PROJECTDIR}`, `${SCDEV_DOMAIN}`, `${SCDEV_HOME}` + all host env vars.

**Volumes** - bind mounts (start with `/`, `.`, `${`) sync host dirs into containers. Named volumes
(just a name like `db_data`) are Docker-managed persistent storage. Named volumes auto-discovered,
persist across stop/start, removed with `down -v`.

**Routing** - HTTP: automatic HTTPS subdomain. TCP: `{ protocol: tcp, port: 5432, host_port: 5432 }`
exposes raw TCP on host (for DB tools). Services within a project reach each other by name.

**Mutagen** (macOS) - fast file sync. Always ignore dependency dirs (`node_modules`, `vendor`) and
build artifacts. Without ignores, installs take 5-10x longer.

For stack-specific config examples, read `references/config-examples.md`.

## Custom Commands (Justfiles)

`.scdev/commands/*.just` files become `scdev` subcommands. The filename is the command name.
Uses [just](https://github.com/casey/just) syntax. `default` recipe runs when no recipe specified.

```
.scdev/commands/
  setup.just   ->  scdev setup
  test.just    ->  scdev test [recipe]
```

```just
# .scdev/commands/test.just
default:
    scdev exec app pnpm test

watch:
    scdev exec app pnpm test --watch
```

Justfiles run on the **host**. Always use `scdev exec` for container commands.

## Setting Up scdev for an Existing Project

1. **Analyze the stack** - check `package.json`, `composer.json`, etc.
2. **Create `.scdev/config.yaml`** - pick image, dev command, port, DB services. See `references/config-examples.md` for templates.
3. **Add to `.gitignore`:** `.scdev/local/` and `.pnpm-store/`
4. **`scdev start`**
5. **Add to README** - `scdev start && scdev setup` instructions
6. **Add to CLAUDE.md** - `scdev exec app <command>` patterns for agents

## Debugging

**Container crashes:** `scdev logs -f app` to see why. `scdev restart` fixes most transient issues.
`scdev down && scdev start` for a full clean restart.

**Redirects to docs page:** Either the container isn't running or `routing.port` doesn't match the
app's port. For shared service UIs (mail, db, redis), also check `scdev services status` - the
service needs to be running AND the project must have the corresponding `shared.*` option enabled.

**File sync issues (macOS):** `scdev mutagen status` / `scdev mutagen reset`

**DB connection refused:** Use service name (`db`), not `localhost`. Example: `postgres://postgres:postgres@db:5432/app`

**Why `.pnpm-store` must be ignored:** pnpm creates a ~500MB platform-specific store. If synced,
wrong binaries (glibc vs musl) break the container on image changes.

## Creating scdev Templates

Templates enable `scdev create <template> my-app` for one-command project scaffolding.

**For the full template authoring guide, read `references/templates.md`.** Key concepts:

- **`.setup-complete` marker pattern** - solves the container startup vs setup circular dependency.
  Container waits in a loop until setup creates the marker, then starts the app.
- **`setup.just`** runs on the host with `scdev exec` for container commands. Interactive terminal
  means framework prompts work (unlike the container entrypoint which has no TTY).
- **Scaffold in-place** (`--force`) when the framework supports it (Nuxt). Scaffold in `/tmp` and
  copy back when the tool requires an empty dir (Symfony) - safe for PHP, not for Node.js/pnpm.
- **`pnpm approve-builds --all`** after install for native module prebuilt binaries (pnpm v10).
- **`npx nuxi prepare`** for Nuxt to trigger module dependency prompts during setup (not at runtime).

Test locally: `scdev create ./my-template test-app && cd test-app && scdev setup`
