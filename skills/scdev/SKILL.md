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

# Print a visually distinct progress marker (for use inside .scdev/commands/*.just)
scdev step "Installing dependencies"   # 2 blank lines + cyan ▶ + bold text;
                                       # makes phase headers pop against composer/npm/apk
                                       # output. Auto-plain on non-TTY / NO_COLOR.

# Logs and info
scdev logs [service]             # First service if omitted. -f to follow, --tail N to limit
scdev info / status / config     # Project info, status, resolved config
scdev open [project]             # Open project URL in browser (current project, or a registered one by name)
scdev rename <new-name>          # Migrate containers/volumes/network/link memberships to a new name

# Shared services
scdev mail / db / redis          # Open in browser
scdev services status            # Check shared services
scdev services recreate          # Rebuild shared containers

# Cross-project networks (links) — see "Linking projects" below
scdev link create <name>         # Make a shared Docker network
scdev link join <name> <proj>[.<svc>] ...   # Attach a whole project or one service
scdev link leave <name> <member> ...
scdev link ls / status <name>    # Inspect
scdev link delete <name>         # Remove network + disconnect members

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

**Multiple routed services in one project** - every `routing:` block defaults to the project's
single domain. A second routed service (e.g. RabbitMQ management UI, a separate admin app) will
collide on the same host. Give it a distinct hostname via `routing.domain`:

```yaml
services:
  rabbitmq:
    image: rabbitmq:3-management-alpine
    routing:
      port: 15672
      domain: rabbitmq.${PROJECTNAME}.${SCDEV_DOMAIN}   # https://rabbitmq.my-app.scalecommerce.site
```

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

## Linking projects

Each scdev project has its own isolated Docker network, so by default two projects can't talk to
each other. When they need to (a monolith calling a microservice you're also developing locally,
split front-end/back-end, shared gateway), create a named **link** — a shared Docker network that
any project can join.

```bash
scdev link create backend              # Create a shared network (Docker: scdev_link_backend)
scdev link join backend api            # Attach all services of project "api"
scdev link join backend worker.app     # Or just one service — project.service notation
scdev link status backend              # Show members + running state
scdev link leave backend api           # Detach; scdev link delete backend tears down the network
```

**DNS on link networks:** across a link, containers are reachable by their fully-qualified name
`<service>.<project>.scdev` (e.g. `app.api.scdev`, `db.api.scdev`). The short service name (`app`,
`db`) is a DNS alias scdev only injects on *the project's own* network — on a link network multiple
projects could have a service called `app`, so always use the long name for cross-project calls.

**Cross-project calls are HTTP to the internal port, not HTTPS on 443.** The public
`https://*.scalecommerce.site` domains resolve to `127.0.0.1`, which from inside a container means
the container's own loopback — not the host's Traefik. TLS termination and Traefik routing only
exist host-side. From one container to another, go direct:

```
http://app.api.scdev:8000      # ✓ cross-project via link (FQDN + internal port)
http://app:8000                # ✓ same project only (short alias)
https://api.scalecommerce.site # ✗ resolves to 127.0.0.1, hits container's own loopback
```

When wiring one project's URL into another's env, use the internal port the target service listens
on (the value of its `routing.port`), not 443.

**Persistence:** link membership is stored in scdev state. `scdev start` auto-reconnects a project
to every link it's a member of — you don't re-run `scdev link join` after a restart, down, or reboot.
`scdev rename` migrates memberships to the new name too. `scdev down` disconnects the project from
its links (containers are gone); the link itself survives until `scdev link delete`.

**Member granularity:** joining a whole project attaches every service container; joining
`project.service` attaches only that one. Use per-service joins when exposing a narrow surface
(e.g. only the API gateway, not the DB) across projects.

## Setting Up scdev for an Existing Project

Different from template scaffolding: the source code is already on the host, so the container's
`command:` doesn't need the `.setup-complete` wait loop — just install deps and exec the dev
server. `setup.just` is optional; only create one if there's a multi-step onboarding (migrations,
seed data, asset build).

1. **Read the existing stack.** `package.json` / `composer.json` / `requirements.txt` for framework
   and dev command. `.env` / `.env.example` for `DATABASE_URL`, `MAILER_DSN`, `HOST`, `PORT`,
   `APP_URL`. Existing `docker-compose.yml` / `Dockerfile` as a hint (ports, volumes) — not source
   of truth. README for any manual setup steps.
2. **Pick a starting config** from `references/config-examples.md` matching the stack.
3. **Bind the dev server to all interfaces** in the `command:` — otherwise the container port isn't
   reachable from Traefik: `HOST=0.0.0.0` (Node), `--host 0.0.0.0` (Vite), `--allow-all-ip`
   (Symfony CLI), `0.0.0.0` (Django `runserver`).
4. **Rewire env for the scdev network.** Change `DATABASE_URL` host from `localhost` / `127.0.0.1`
   / compose's service name to the scdev service name you declared (commonly `db`). Set
   `MAILER_DSN: "smtp://mail:1025"` so outbound mail is caught by Mailpit. **For any Symfony /
   Sylius / Shopware / Laravel project**, also set `SYMFONY_TRUSTED_PROXIES: private_ranges` now —
   without it, the debug toolbar and login flows break behind Traefik (see Debugging → "Symfony/Sylius
   behind Traefik"). Laravel equivalent: `TRUSTED_PROXIES=*`.
5. **Mirror the existing DB** (image, version, credentials, database name) — the app's `.env`
   usually tells you all three.
6. **`mutagen.ignore`** for dependency and build artifact dirs: `node_modules`, `.pnpm-store`,
   `vendor`, `var/cache`, `.nuxt`, `.next`, framework-specific build output. Without this, installs
   are 5–10× slower on macOS.
7. **`.gitignore`:** add `.scdev/local/` (always) and `.pnpm-store/` (pnpm projects).
8. **`scdev start`, then verify in a browser** (not just curl): check the console and network tabs
   for mixed-content, missing assets, JS errors. `curl 200` lies for HTML apps.
9. **Document** `scdev start` in the project README and `scdev exec app <cmd>` patterns in its
   `CLAUDE.md` so future agents know how to operate the env.

For stack-specific landmines when writing the config or entrypoint (Node corepack, pnpm build
scripts, PHP `memory_limit`, PHP extensions, Symfony `TRUSTED_PROXIES`, Webpack Encore / Vite
rebuild, mailer DSN), read `references/stack-gotchas.md` — those patterns apply whether you're
wrapping an existing repo or authoring a template.

## Debugging

**Container crashes:** `scdev logs -f app` to see why. `scdev restart` fixes most transient issues.
`scdev down && scdev start` for a full clean restart.

**Config changes aren't taking effect:** `scdev restart` just stops+starts the existing container,
so edits to `environment:`, `image:`, `command:`, routing, or volume mounts don't apply. Use
`scdev update` — it diffs the config against the running containers and recreates only the services
that actually changed. Code changes are live via bind mount / Mutagen — no restart or update needed.

**Redirects to docs page:** Either the container isn't running or `routing.port` doesn't match the
app's port. For shared service UIs (mail, db, redis), also check `scdev services status` - the
service needs to be running AND the project must have the corresponding `shared.*` option enabled.

**File sync issues (macOS):** `scdev mutagen status` / `scdev mutagen reset`

**DB connection refused:** Use service name (`db`), not `localhost`. Example: `postgres://postgres:postgres@db:5432/app`

**Can't reach `https://other-project.scalecommerce.site` from inside a container:** `*.scalecommerce.site`
is a wildcard that resolves to `127.0.0.1` — from inside a container that's its own loopback, not
the host's Traefik. Container-to-container traffic must go HTTP-direct using the container name:
`http://<service>.<project>.scdev:<internal-port>`. Both projects must be joined to a shared link
(see "Linking projects").

**Symfony/Sylius behind Traefik — stuck "Loading…" debug toolbar, broken admin login, mixed-content
errors in the console:** Traefik terminates TLS and forwards HTTP to the app. Without trusted-proxy
config, Symfony can't tell the outer request was HTTPS and generates `http://` URLs inside the HTTPS
page — the browser blocks them. Fix: add `SYMFONY_TRUSTED_PROXIES: private_ranges` to the app service
`environment:` (Symfony 6.3+ shorthand for RFC1918 + 127.0.0.1). Laravel equivalent: `TRUSTED_PROXIES=*`
for the TrustProxies middleware. Any framework that generates absolute URLs needs similar awareness.

**`curl 200 OK` isn't enough for HTML apps:** mixed-content, CSP failures, and JS errors are
browser-only failure modes — curl can't see them. After finishing a web template or UI change, open
the project in a browser via the chrome-devtools MCP tools (`new_page`, `list_console_messages`,
`list_network_requests`) and confirm the console is clean and all requests are `https://`.

**Why `.pnpm-store` must be ignored:** pnpm creates a ~500MB platform-specific store. If synced,
wrong binaries (glibc vs musl) break the container on image changes.

## Creating scdev Templates

Templates enable `scdev create <template> my-app` for one-command project scaffolding.

**For template authoring: read `references/templates.md`** for the `.setup-complete` pattern,
scaffolding strategies, and `setup.just` conventions. For the stack-specific runtime behaviors the
template entrypoint must handle (Node, PHP, PHP framework landmines), read
`references/stack-gotchas.md`. Key concepts:

- **`.setup-complete` marker pattern** - solves the container startup vs setup circular dependency.
  Container waits in a loop until setup creates the marker, then starts the app.
- **`setup.just`** runs on the host with `scdev exec` for container commands. Interactive terminal
  means framework prompts work (unlike the container entrypoint which has no TTY).
- **`@scdev step "<msg>"` for phase headers** in setup.just, not `@echo`. Setup output is a wall of
  apk/composer/npm noise; plain echo disappears in it. `scdev step` prints two blank lines + cyan
  `▶` + bold so each phase stands out. Auto-plain on non-TTY / `NO_COLOR`.
- **Scaffold in-place** (`--force`) when the framework supports it (Nuxt). Scaffold in `/tmp` and
  copy back when the tool requires an empty dir (Symfony) - safe for PHP, not for Node.js/pnpm.
- **`pnpm approve-builds --all`** after install for native module prebuilt binaries (pnpm v10).
- **`npx nuxi prepare`** for Nuxt to trigger module dependency prompts during setup (not at runtime).

Test locally: `scdev create ./my-template test-app && cd test-app && scdev setup`
