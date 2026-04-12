# scdev

**Ever seen a developer and an AI agent fall in love with a dev environment?** 🧑‍💻🤖❤️

scdev is a local development tool that gets you from `git clone` to coding in seconds. One command starts your entire project - HTTPS, routing, shared services, and all. Simple enough for any AI coding agent to operate, powerful enough for complex multi-service setups.

```bash
cd my-project
scdev start
# Your project is running at https://my-project.scalecommerce.site
```

> `scalecommerce.site` is a wildcard DNS pointing to `127.0.0.1` - everything runs locally on your machine. No cloud, no accounts. You can use your own domain too.

**Requires:** [Docker Desktop](https://www.docker.com/products/docker-desktop/) (macOS/Windows) or Docker Engine (Linux)

## How It Works

Every project runs in its own isolated network. scdev gives each project its own HTTPS subdomain - no port conflicts, no SSL setup. Shared services like mail catching, database browsing, and Redis inspection are available to all projects automatically.

> [!IMPORTANT]
> **Your code runs in containers, not on your machine.** Every `pnpm install`, `composer install`, and dev server runs inside an isolated Docker container. If a malicious npm package tries to steal your SSH keys, read your browser cookies, or encrypt your files - it can't. It's trapped in a throwaway container with no access to your host. In an era where supply chain attacks on npm, PyPI, and Packagist are increasingly common, this isn't just convenience - it's protection.

![scdev architecture](docs/architecture.png)

## Built for Coding Agents

scdev gives AI coding agents (Claude Code, Cursor, Copilot) exactly what they need: deterministic environments with zero ambiguity.

- **One command** - `scdev start` is all the agent needs. No multi-step setup to get wrong.
- **Predictable URLs** - The app is always at `https://{name}.scalecommerce.site`. No port guessing.
- **Single config file** - `.scdev/config.yaml` is the complete source of truth. One file to read, not five.
- **Discoverable commands** - `ls .scdev/commands/` reveals all project-specific tasks. No guessing.
- **`scdev exec app <cmd>`** - Run anything in any container. No container name lookup needed.

### Agent Integration

Install the scdev skill so your agent knows how to use the dev environment:

```bash
npx skills add scalecommerce-dev/scdev
```

This teaches your agent the full scdev CLI, config format, debugging workflows, and project setup patterns. Your agent can also help you create custom scdev [templates](#templates).

## Why scdev?

| Without scdev | With scdev |
|---------------|------------|
| Port conflicts between projects | Every project gets its own HTTPS subdomain |
| Each project configures its own mail, DB tools | Shared services run once, work for all projects |
| New developer spends a day setting up | Clone, `scdev start`, done |
| Complex Docker Compose with 100+ lines | Simple config with sensible defaults |
| Slow file sync on macOS | Native-speed file sync, zero config |
| Malicious packages can access your entire machine | Code runs in isolated containers - supply chain attacks stay sandboxed |

## Quick Start

### 1. Install

```bash
curl -fsSL https://raw.githubusercontent.com/ScaleCommerce-DEV/scdev/main/install.sh | sh
```

### 2. First-time setup

This installs SSL certificates and starts the shared services (router, mail catcher, DB browser):

```bash
scdev systemcheck
```

### 3. Create a project

The fastest way is to use a template:

```bash
scdev create express my-app
cd my-app
scdev setup
```

Open https://my-app.scalecommerce.site - that's it. HTTPS works out of the box.

Or create a project manually with a config file at `my-app/.scdev/config.yaml`:

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
    - .nuxt
```

`${PROJECTPATH}` is resolved automatically to your project's absolute path. Other available variables: `${PROJECTNAME}`, `${PROJECTDIR}`, `${SCDEV_DOMAIN}`.

```bash
cd my-app
scdev start
```

## Templates

Create new projects from starter templates with `scdev create`:

```bash
scdev create express my-app            # Express.js
scdev create nuxt4 my-app              # Nuxt 4
scdev create symfony my-app            # Symfony
scdev create myorg/my-template my-app  # Any GitHub repo
```

Browse all available templates on GitHub: [ScaleCommerce-DEV repositories matching `scdev-template-`](https://github.com/orgs/ScaleCommerce-DEV/repositories?q=scdev-template-). Each template's README explains what it includes and how to use it.

Want to create your own template? See the [Template Authoring Guide](templates/README.md).

## Shared Services

These run once and are shared across all your projects. No per-project configuration needed.

| Service | URL | What it does |
|---------|-----|--------------|
| Router | `https://router.shared.scalecommerce.site` | Routing dashboard - see all routes |
| Mail | `https://mail.shared.scalecommerce.site` | Catches all outgoing email ([Mailpit](https://github.com/axllent/mailpit)) |
| DB | `https://db.shared.scalecommerce.site` | Browse any project's database ([Adminer](https://www.adminer.org/)) |
| Redis | `https://redis.shared.scalecommerce.site` | Inspect Redis keys and data ([Redis Insights](https://redis.io/insight/)) |

**Connecting from your app containers:** Configure your app to send mail to `mail:1025` (SMTP, no auth). For databases and Redis, use your project's own service names (e.g., `db:5432`, `redis:6379`) - Adminer and Redis Insights are browser UIs, not the services themselves.

Open them directly:

```bash
scdev mail    # open Mailpit
scdev db      # open Adminer
scdev redis   # open Redis Insights
```

## Features

### Automatic HTTPS

Every project and shared service gets locally-trusted HTTPS certificates. Your browser shows a green lock, cookies work with `Secure` flag, and your local environment matches production.

### Fast File Sync (macOS)

File sharing between your host and containers is notoriously slow on macOS. scdev automatically syncs files at native speed - no configuration needed. On Linux this isn't needed (already fast).

How much difference does it make? We benchmarked a Nuxt 4 app with ~1000 dependencies:

| Approach | pnpm install | Cold start to app ready |
|----------|-------------|------------------------|
| Docker bind mount (default macOS) | **34.6s** | ~42s |
| scdev with file sync | **6.7s** | ~17s |
| scdev warm restart (stop + start) | **2.4s** | ~2s |

That's a **5x speedup** on cold start and **instant** warm restarts. The trick: scdev syncs your source code via fast file sync, while keeping `node_modules` and other generated files inside the container where filesystem operations are native speed.

Exclude paths you don't need synced back to the host:

```yaml
mutagen:
  ignore:
    - node_modules
    - .pnpm-store    # pnpm's content-addressable store - platform-specific, don't sync
    - .nuxt
    - .output
    - var/cache
```

**Important:** Always add `.pnpm-store` to the ignore list for pnpm projects. pnpm creates its package store inside the project directory when running in a container. Without ignoring it, ~500MB of platform-specific binaries sync to the host, causing slow syncs and broken native modules when switching images.

### Multi-Service Routing

By default, all HTTP services in a project share the project's domain. For projects with multiple web services (frontend + backend, app + admin), you can assign each service its own domain using `routing.domain`:

```yaml
name: my-app

services:
  frontend:
    image: node:22-alpine
    command: pnpm dev --host 0.0.0.0
    working_dir: /app
    volumes:
      - ${PROJECTPATH}/frontend:/app
    routing:
      port: 3000
      # Uses project domain: my-app.scalecommerce.site

  backend:
    image: node:22-alpine
    command: pnpm dev --host 0.0.0.0
    working_dir: /app
    volumes:
      - ${PROJECTPATH}/backend:/app
    routing:
      port: 4000
      domain: api.${PROJECTNAME}.${SCDEV_DOMAIN}
      # Uses custom domain: api.my-app.scalecommerce.site
```

The `domain` field supports variable substitution and only applies to HTTP/HTTPS routing (not TCP/UDP).

### TCP/UDP Routing

Beyond HTTPS, scdev can expose raw TCP and UDP ports. This lets you connect to a database inside a project from your host using tools like DBeaver, pgAdmin, or `psql`:

```yaml
services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_PASSWORD: postgres
    routing:
      protocol: tcp
      port: 5432        # container port
      host_port: 5432   # exposed on localhost:5432
```

```bash
psql -h localhost -p 5432 -U postgres   # connect from your host
```

Multiple projects can expose different ports without conflicts. Works for MySQL, Redis, RabbitMQ, or any TCP/UDP service.

### Volumes

**Bind mounts** (`${PROJECTPATH}:/app`) sync your source code into the container. Edits on the host are reflected immediately. On macOS, scdev handles fast sync automatically via Mutagen. Add `node_modules`, `.pnpm-store`, and build caches to `mutagen.ignore` so they stay inside the container (fast) and don't sync back to the host.

**Named volumes** (`db_data:/var/lib/postgresql/data`) are persistent storage managed by scdev. Use these for data that must survive `scdev down` - database files, uploaded assets, SQLite databases:

```yaml
volumes:
  - ${PROJECTPATH}:/app              # your source code (synced to host)
  - db_data:/var/lib/postgresql/data  # database files (persists across down)
  - data:/app/data                   # SQLite, uploads, etc.
```

Named volumes persist across `scdev stop`/`scdev start` AND `scdev down`. Only removed with `scdev down -v`. No separate declaration needed - scdev discovers them automatically.

### Custom Commands

Every project has recurring tasks: install deps, run migrations, seed data, run tests. Instead of documenting these in a README, define them as [just](https://github.com/casey/just) files in `.scdev/commands/`. The filename becomes the command:

```
.scdev/commands/
  setup.just     ->  scdev setup
  test.just      ->  scdev test
  seed.just      ->  scdev seed
```

```bash
# .scdev/commands/setup.just
default:
    scdev exec app pnpm ci
    scdev exec app npx prisma db push

# .scdev/commands/test.just
default:
    scdev exec app pnpm test

watch:
    scdev exec app pnpm test -- --watch
```

```bash
scdev setup          # install deps + push schema
scdev test           # run tests
scdev test watch     # run tests in watch mode
```

Agents can `ls .scdev/commands/` to discover all available project tasks.

### Project Isolation

Each project runs in its own isolated network. Services within a project reach each other by name (`db`, `redis`, `app`), but projects can't see each other's services. The shared router bridges them to the outside.

## Commands

### Lifecycle

```bash
scdev start       # Start the project
scdev stop        # Stop containers (keeps them for quick restart)
scdev restart     # Stop + start
scdev down        # Remove containers and network
scdev down -v     # Remove everything including volumes
scdev rename <n>  # Rename project, migrate volumes, restart
```

### Development

```bash
scdev exec app bash              # Shell into a container
scdev exec app pnpm test         # Run a command
scdev logs                       # View logs
scdev logs -f app                # Follow logs for a service
```

### Information

```bash
scdev info        # Show project info, URLs, services
scdev list        # List all projects
scdev config      # Show resolved configuration
scdev status      # Quick status check
```

### Shared Services

```bash
scdev services status    # Check shared service status
scdev services start     # Start shared services
scdev services stop      # Stop shared services
scdev services recreate  # Rebuild shared service containers
```

### Link Networks

Link networks enable direct container-to-container communication between separate projects. Each project runs on its own isolated Docker network, so by default containers in project A cannot reach containers in project B. Link networks solve this by creating a shared Docker network that selected containers join.

```bash
scdev link create <name>                        # Create a named link network
scdev link join <name> <member> [<member>...]   # Add projects or services
scdev link leave <name> <member> [<member>...]  # Remove members
scdev link delete <name>                        # Remove link and disconnect all
scdev link ls                                   # List all links
scdev link status <name>                        # Show members and connection state
```

Members can be whole projects or individual services:

```bash
scdev link create backend-mesh
scdev link join backend-mesh sec-scan sec-scan-decoder
scdev link join backend-mesh redis-debug.app    # only the app service
```

Linked containers reach each other by their **container name**, not the project domain:

```bash
# From inside sec-scan, reach sec-scan-decoder's app service:
curl http://app.sec-scan-decoder.scdev:3000
```

**Why container names, not project domains?** The project domain (e.g., `sec-scan-decoder.scalecommerce.site`) uses wildcard DNS that resolves to `127.0.0.1`. Inside a container, `127.0.0.1` points to the container itself, not the host or Traefik - so the domain is unreachable. Container names (e.g., `app.sec-scan-decoder.scdev`) are resolved by Docker's built-in DNS, which returns the actual container IP on the shared link network. This works reliably and without TLS certificate issues.

The container name pattern is `<service>.<project>.scdev` - the same name shown by `scdev link status`.

Links are stored in the global state file and survive restarts - when a linked project starts, its containers are automatically reconnected to the link network. Each link creates its own Docker network (`scdev_link_<name>`), so different link groups stay isolated from each other.

Link names may only contain alphanumeric characters, hyphens, and underscores.

### File Sync (macOS)

```bash
scdev mutagen status  # Check sync status
scdev mutagen flush   # Wait for sync to complete
scdev mutagen reset   # Recreate sync sessions (if stuck)
```

## Examples

### PHP + MySQL

```yaml
name: my-shop

variables:
  DB_PASSWORD: root
  DB_NAME: ${PROJECTNAME}

services:
  app:
    image: webdevops/php-nginx:8.2
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app
    environment:
      WEB_DOCUMENT_ROOT: /app/public
      DATABASE_URL: mysql://root:${DB_PASSWORD}@db:3306/${DB_NAME}
    routing:
      port: 80

  db:
    image: mysql:8.0
    volumes:
      - db_data:/var/lib/mysql
    environment:
      MYSQL_ROOT_PASSWORD: ${DB_PASSWORD}
      MYSQL_DATABASE: ${DB_NAME}
```

### Node.js + PostgreSQL

```yaml
name: my-api

services:
  app:
    image: node:22-alpine
    command: corepack enable && pnpm install && pnpm dev --host 0.0.0.0
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app
    environment:
      DATABASE_URL: postgres://postgres:postgres@db:5432/app
    routing:
      port: 3000

  db:
    image: postgres:16-alpine
    volumes:
      - db_data:/var/lib/postgresql/data
    environment:
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: app

mutagen:
  ignore:
    - node_modules
    - .pnpm-store
    - .nuxt
```

### Static Site / Frontend

```yaml
name: my-docs

services:
  app:
    image: node:22-alpine
    command: corepack enable && pnpm install && pnpm dev --host 0.0.0.0
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app
    routing:
      port: 5173

mutagen:
  ignore:
    - node_modules
    - .pnpm-store
```

## Configuration Reference

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

### Project Configuration Reference (`.scdev/config.yaml`)

#### Project-level fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | directory name | Project name, used in domain and container names |
| `domain` | string | `{name}.scalecommerce.site` | Project domain for HTTP routing |
| `variables` | map | - | Reusable `${VAR}` placeholders substituted throughout the config (not passed to containers) |
| `environment` | map | - | Environment variables passed to ALL containers |
| `shared.router` | bool | `true` | Connect to shared Traefik router |
| `shared.mail` | bool | `false` | Connect to shared Mailpit |
| `shared.db` | bool | `false` | Connect to shared Adminer |
| `shared.redis` | bool | `false` | Connect to shared Redis Insights |
| `mutagen.ignore` | list | - | Paths excluded from file sync (macOS). Mutagen itself is configured globally, see below |

#### Service fields (`services.<name>.`)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `image` | string | required | Docker image |
| `command` | string | - | Container command |
| `working_dir` | string | - | Working directory inside container |
| `volumes` | list | - | Volume mounts (bind mounts and named volumes) |
| `environment` | map | - | Env vars for this container (overrides project-level) |
| `routing.protocol` | string | `http` | `http`, `https`, `tcp`, `udp` |
| `routing.port` | int | 80 (http), 443 (https) | Container port to route to |
| `routing.host_port` | int | - | Host port for TCP/UDP (required for tcp/udp) |
| `routing.domain` | string | project domain | Custom domain for this service (http/https only) |
| `pre_start` | list | - | Commands to run before container starts |
| `labels` | map | - | Docker labels |

#### Variables and environment

**`variables`** define `${VAR}` placeholders substituted throughout the config file. They are NOT passed to containers. Use them to avoid duplicating values like database passwords across services.

**`environment`** (project-level) is passed to ALL containers. **`services.<name>.environment`** is passed to that specific container and overrides project-level values with the same name.

**Built-in variables:** `${PROJECTNAME}`, `${PROJECTPATH}`, `${PROJECTDIR}`, `${SCDEV_DOMAIN}`, `${SCDEV_HOME}`, `${USER}`, `${HOME}`, plus all host environment variables. User-defined `variables` can reference built-in ones (e.g. `DB_NAME: ${PROJECTNAME}_db`).

### Global Configuration Reference (`~/.scdev/global-config.yaml`)

Applies to all projects. Auto-created on first run. Usually you don't need to touch this.

```yaml
domain: scalecommerce.site
ssl:
  enabled: true
mutagen:
  enabled: auto       # "auto" (macOS only), "true" (always), "false" (never)
  sync_mode: two-way-safe  # default sync mode
```

## Troubleshooting

### "DNS doesn't resolve"

`scalecommerce.site` uses wildcard DNS pointing to `127.0.0.1`. If it doesn't work:

1. Check: `dig my-app.scalecommerce.site`
2. Corporate VPNs sometimes block external DNS - try a different network
3. Add entries to `/etc/hosts` as a workaround

### "Containers won't start"

```bash
scdev down           # clean up
scdev start          # try again
scdev logs -f app    # check what's happening
```

### "File sync is slow" (macOS)

```bash
scdev mutagen status   # check if Mutagen is running
scdev mutagen reset    # recreate sync sessions if stuck
```

### "Port already in use"

scdev uses ports 80 and 443 for the shared router. Check what's using them:

```bash
lsof -i :80
lsof -i :443
```

## Standing on the Shoulders of Giants

scdev doesn't reinvent the wheel. It orchestrates proven open-source tools into a seamless experience - so you get the power without the configuration.

| Technology | What scdev uses it for | Link |
|------------|----------------------|------|
| [Docker](https://www.docker.com/) | Container runtime, network isolation | docker.com |
| [Traefik](https://traefik.io/) | Reverse proxy - HTTPS routing, subdomains, TCP/UDP | traefik.io |
| [mkcert](https://github.com/FiloSottile/mkcert) | Locally-trusted SSL certificates | github.com/FiloSottile/mkcert |
| [Mutagen](https://mutagen.io/) | Fast file sync on macOS | mutagen.io |
| [just](https://github.com/casey/just) | Command runner for project tasks | github.com/casey/just |
| [Mailpit](https://github.com/axllent/mailpit) | Email testing - catches all outgoing mail | github.com/axllent/mailpit |
| [Adminer](https://www.adminer.org/) | Database browser - MySQL, PostgreSQL, SQLite | adminer.org |
| [Redis Insights](https://redis.io/insight/) | Redis browser - keys, queries, memory analysis | redis.io/insight |

## Contributing

Want to help improve scdev? See [CONTRIBUTING.md](CONTRIBUTING.md) for the developer guide - project structure, testing strategy, architecture decisions, and how to add new features.

Want to create a project template? See the [Template Authoring Guide](templates/README.md).

## License

MIT
