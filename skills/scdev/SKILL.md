---
name: scdev
description: |
  Local development environment framework using Docker. Use this skill whenever the user's project
  has a `.scdev/` directory, when they ask to set up a local dev environment, start/stop containers,
  run commands inside containers, check logs, configure routing, or debug container issues.
  Also trigger when: the user says "start the project", "run tests in docker", "check the logs",
  "open mailpit", "set up scdev", "add scdev to this project", mentions `.scdev/config.yaml`,
  or asks about shared services, HTTPS certificates, or file sync on macOS.
---

# scdev - Local Development Environment

scdev manages Docker-based local dev environments. One command starts an entire project with HTTPS,
routing, shared services (mail, DB browser, Redis browser), and container isolation.

## How It Works

- Each project has a `.scdev/config.yaml` that defines its services (containers)
- `scdev start` creates an isolated Docker network, starts containers, connects shared services
- A shared Traefik router gives each project its own HTTPS subdomain
- `*.scalecommerce.site` is a wildcard DNS pointing to `127.0.0.1` - everything runs locally
- SSL certificates are auto-generated via mkcert and trusted by the local system
- On macOS, Mutagen provides fast file sync (replaces slow Docker bind mounts)

## Prerequisites

The `scdev` binary must be installed:

```bash
scdev version
```

If not found, tell the user:

> Install scdev:
> ```bash
> curl -fsSL https://raw.githubusercontent.com/ScaleCommerce-DEV/scdev/main/install.sh | sh
> scdev systemcheck
> ```

## CLI Reference

### Project Lifecycle

```bash
scdev start              # Start project (creates network, volumes, containers, connects shared services)
scdev stop               # Stop containers (preserves state for fast restart)
scdev restart            # Stop + start
scdev down               # Remove containers and network
scdev down -v            # Remove everything including volumes
scdev update             # Recreate containers that changed in config
```

### Running Commands in Containers

```bash
scdev exec <service> <command>     # Run a command in a container
scdev exec app bash                # Interactive shell
scdev exec app pnpm test            # Run tests
scdev exec app php artisan migrate # Run migrations
scdev exec app npx prisma db push  # Push DB schema
```

`scdev exec` is the primary way to run anything inside a container. The service name matches
the key in `.scdev/config.yaml` (e.g., `app`, `db`, `worker`).

### Logs and Debugging

```bash
scdev logs                # Logs from first service
scdev logs app            # Logs from specific service
scdev logs -f app         # Follow logs in real-time
scdev logs --tail 50 app  # Last 50 lines
scdev info                # Show project URLs, services, volumes, shared connections
scdev status              # Quick status check
scdev config              # Show resolved config (after variable substitution)
```

### Shared Services

These run once and are shared by all projects:

| Service | Command | URL | Purpose |
|---------|---------|-----|---------|
| Mail | `scdev mail` | `https://mail.shared.scalecommerce.site` | Catches all outgoing email (Mailpit) |
| DB | `scdev db` | `https://db.shared.scalecommerce.site` | Database browser (Adminer) |
| Redis | `scdev redis` | `https://redis.shared.scalecommerce.site` | Redis browser (Redis Insights) |
| Docs | `scdev docs` | `https://docs.shared.scalecommerce.site` | scdev docs + project list |

```bash
scdev services status    # Check what's running
scdev services start     # Start all shared services
scdev services recreate  # Rebuild shared service containers
```

### Other Commands

```bash
scdev list               # List all registered projects
scdev volumes            # List project volumes
scdev self-update        # Update scdev binary
```

### Global Flag

```bash
scdev --config /path/to/project <command>   # Use config from a different directory
```

## Project Configuration

### Minimal `.scdev/config.yaml`

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

This gives you: `https://my-app.scalecommerce.site` with HTTPS, isolated network, shared services.

**Notes:**
- `version` and `domain` are optional - scdev defaults to `version: 1` and `{name}.scalecommerce.site`
- `protocol` defaults to `http` when `port` is set
- On macOS, scdev's sync-ready gate automatically holds the command until files are synced -
  no need for `while [ ! -f ... ]` workarounds

### Available Variables

| Variable | Value |
|----------|-------|
| `${PROJECTPATH}` | Absolute path to the project directory |
| `${PROJECTNAME}` | Project name from config |
| `${PROJECTDIR}` | Directory basename |
| `${SCDEV_DOMAIN}` | Base domain (default: `scalecommerce.site`) |
| `${SCDEV_HOME}` | `~/.scdev` path |

### Full Config Options

```yaml
name: my-project

shared:
  router: true          # connect to Traefik (default: true)
  mail: true            # connect to Mailpit
  db: true              # connect to Adminer
  redis_insights: true  # connect to Redis Insights

environment:            # global env vars for all services
  APP_ENV: dev

services:
  app:
    image: node:22-alpine
    command: corepack enable && pnpm install && pnpm dev --host 0.0.0.0
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app        # bind mount (auto-synced via Mutagen on macOS)
    environment:
      DATABASE_URL: postgres://postgres:postgres@db:5432/app
    routing:
      protocol: http    # http (default), https, tcp, udp
      port: 3000        # container port to route to

  db:
    image: postgres:16-alpine
    volumes:
      - db_data:/var/lib/postgresql/data
    environment:
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: app

mutagen:
  ignore:               # paths excluded from file sync (macOS)
    - node_modules
    - .pnpm-store       # pnpm's package store - platform-specific, must not sync
    - .nuxt
    - .output
```

### Routing

**HTTP/HTTPS** - Traefik routes by hostname. Project gets `https://{name}.scalecommerce.site`:

```yaml
routing:
  protocol: http   # default when port is set
  port: 3000       # container port
```

**TCP passthrough** - Exposes a raw TCP port on the host via Traefik. This is powerful because
it lets you connect to a database inside Docker from host tools (DBeaver, pgAdmin, `psql`) or
even from another scdev project:

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

Now you can connect from your host machine:

```bash
psql -h localhost -p 5432 -U postgres    # connect from host
```

Or from another scdev project's container (via the shared router):

```bash
# From project B, connect to project A's database
scdev exec app psql -h scdev_router -p 5432 -U postgres
```

This also works for MySQL, Redis, RabbitMQ, or any TCP service. Multiple projects can expose
different ports without conflicts:

```yaml
# Project A: PostgreSQL on 5432
routing: { protocol: tcp, port: 5432, host_port: 5432 }

# Project B: MySQL on 3306
routing: { protocol: tcp, port: 3306, host_port: 3306 }
```

**UDP passthrough** - Same as TCP but for UDP (e.g., DNS, game servers, QUIC):

```yaml
routing:
  protocol: udp
  port: 53
  host_port: 5353
```

**Internal-only services** - Services without `routing` are only reachable within their
project's network. This is the default for databases, caches, and workers - they don't need
external access.

Services within a project always reach each other by name: `db`, `redis`, `app`, etc.

### Volumes: Bind Mounts vs Named Volumes

There are two types of volume mounts in scdev. Use the right one for the job:

**Bind mounts** - sync a host directory into the container. Starts with `/`, `.`, or `${...}`:

```yaml
volumes:
  - ${PROJECTPATH}:/app              # your source code
  - ./config/nginx.conf:/etc/nginx/nginx.conf  # single file mount
```

Use for: **source code and config files** - anything you edit on the host and want reflected
inside the container immediately. On macOS, bind mounts are automatically synced via Mutagen
for performance.

**Named volumes** - Docker-managed storage that lives inside Docker. Just a name, no path prefix:

```yaml
volumes:
  - node_modules:/app/node_modules   # dependencies
  - db_data:/var/lib/postgresql/data  # database files
  - composer_cache:/root/.composer    # package cache
```

Use for: **generated data that should NOT sync to the host** - dependencies (`node_modules`,
`vendor`), database files, caches. These are often huge, change constantly, and would destroy
file sync performance if they were bind mounts. They persist across `scdev stop`/`scdev start`
but are removed with `scdev down -v`.

**Common pattern** - bind mount the project, named volume for dependencies:

```yaml
volumes:
  - ${PROJECTPATH}:/app              # source code (synced)
  - node_modules:/app/node_modules   # deps (NOT synced, lives in Docker)
```

This is important: `node_modules` as a named volume "masks" the host's `node_modules` inside
the container. The container has its own copy of dependencies, compiled for its own OS/arch.
Your IDE still sees the host `node_modules` for autocomplete.

Named volumes are auto-discovered from service definitions - no separate declaration needed.
scdev creates them on `start` and removes them on `down -v`.

## Custom Commands (Justfiles)

### Why justfiles?

Every project has recurring tasks: install dependencies, run migrations, seed data, run tests,
create an admin user. Without scdev, these live in READMEs, Makefiles, or tribal knowledge.
Developers (and agents) have to figure out which commands to run, in which container, in which
order.

scdev solves this with [just](https://github.com/casey/just) - a command runner like `make`
but simpler. Each `.just` file in `.scdev/commands/` becomes a top-level scdev command. The
filename IS the command name. This means:

- `scdev setup` - everyone runs the same setup steps
- `scdev test` - no guessing how to run tests
- `scdev seed` - one command to populate the database
- Agents can `ls .scdev/commands/` to discover all available project tasks

### How it works

```
.scdev/commands/
  setup.just       # scdev setup
  test.just        # scdev test
  seed.just        # scdev seed
  admin.just       # scdev admin
```

Each file contains recipes (like Makefile targets). The `default` recipe runs when no
recipe is specified. Additional recipes are passed as arguments.

### Examples

**setup.just** - First-time project setup:

```just
default:
    scdev exec app pnpm ci
    scdev exec app npx prisma db push
    scdev exec app pnpm run build

clean:
    scdev exec app rm -rf node_modules
    scdev down -v
```

```bash
scdev setup          # install deps, push schema, build
scdev setup clean    # nuke everything and start fresh
```

**test.just** - Run tests:

```just
default:
    scdev exec app pnpm test

watch:
    scdev exec app pnpm test -- --watch

coverage:
    scdev exec app pnpm test -- --coverage
```

```bash
scdev test           # run tests once
scdev test watch     # watch mode
scdev test coverage  # with coverage report
```

**seed.just** - Database seeding and user management:

```just
default:
    scdev exec app pnpm run db:seed

admin:
    scdev exec app pnpm run create-admin -- --email admin@example.com

reset:
    scdev exec app npx prisma migrate reset --force
    scdev exec app pnpm run db:seed
```

```bash
scdev seed           # seed test data
scdev seed admin     # create admin user
scdev seed reset     # wipe DB, re-migrate, re-seed
```

**migrate.just** - Database migrations:

```just
default:
    scdev exec app npx prisma migrate dev

status:
    scdev exec app npx prisma migrate status

reset:
    scdev exec app npx prisma migrate reset --force
```

### Environment variables available in justfiles

Justfiles automatically receive these variables:

| Variable | Value |
|----------|-------|
| `PROJECTNAME` | Project name from config |
| `PROJECTPATH` | Absolute path to project |
| `PROJECTDIR` | Directory basename |
| `SCDEV_DOMAIN` | Base domain |
| `SCDEV_HOME` | `~/.scdev` path |
| All `environment:` vars | From project config |

### Key pattern: always use `scdev exec`

Commands in justfiles should use `scdev exec` to run inside containers, not bare commands.
This ensures the command runs in the right environment with the right dependencies:

```just
# Good - runs inside the container
default:
    scdev exec app pnpm test

# Bad - runs on the host, which might not have node/npm
default:
    pnpm test
```

## Setting Up scdev for an Existing Project

### Step 1: Create the config

Analyze the project to build the right config:

1. **Check the tech stack** - look at `package.json`, `composer.json`, `requirements.txt`, `go.mod`
2. **Pick the base image**:
   - Node.js: `node:22-alpine` (or `node:22` if native modules need glibc)
   - PHP: `webdevops/php-nginx:8.3` (with nginx) or `php:8.3-fpm` (FPM only)
   - Python: `python:3.12-slim`
3. **Find the dev command** - check `package.json` scripts, `Makefile`, README
4. **Find the port** - check dev server output or framework defaults (Nuxt: 3000, Vite: 5173, Laravel: 80, Django: 8000)
5. **Add database services** if needed - check for connection strings in `.env` or config
6. **Add mutagen ignores** for the detected stack (see table below)

Create `.scdev/config.yaml`. See "Config Templates by Stack" below for ready-to-use templates.

### Step 2: Add to .gitignore

```
# scdev local state
.scdev/local/
# pnpm store (created inside container, platform-specific)
.pnpm-store/
```

### Step 3: Start

```bash
scdev start
```

### Step 4: Add onboarding instructions to README

Add a section to the project's README so new developers can get running fast:

```markdown
## Local Development

Install [scdev](https://github.com/ScaleCommerce-dev/scdev):

\`\`\`bash
curl -fsSL https://raw.githubusercontent.com/ScaleCommerce-DEV/scdev/main/install.sh | sh
scdev systemcheck   # first time only
\`\`\`

Start the project:

\`\`\`bash
scdev start
scdev setup         # install dependencies, run migrations (if setup.just exists)
\`\`\`

Open https://{project-name}.scalecommerce.site

Useful commands:

\`\`\`bash
scdev exec app bash     # shell into the app container
scdev logs -f app       # follow logs
scdev mail              # open email catcher
scdev db                # open database browser
\`\`\`
```

### Step 5: Add agent instructions to CLAUDE.md (or agents.md)

Add a section so AI coding agents know how to use the dev environment:

```markdown
## Development Environment

This project uses [scdev](https://github.com/ScaleCommerce-dev/scdev) for local development.

- Start: `scdev start`
- Run commands: `scdev exec app <command>`
- View logs: `scdev logs -f app`
- Project URL: https://{project-name}.scalecommerce.site
- Mail catcher: `scdev mail`
- Database browser: `scdev db`

Run tests with `scdev exec app pnpm test` (or `scdev test` if a test.just exists).
```

## Debugging

**Container won't start or keeps crashing:**
```bash
scdev logs -f <service>   # check what's failing
scdev restart             # quick restart (fixes most transient issues)
scdev down && scdev start # full clean restart if restart didn't help
scdev config              # verify resolved config looks right
```
Containers can crash for many reasons - Docker Desktop updates, resource limits, bad commands.
Don't over-investigate. `scdev restart` fixes most cases.

**Can't reach the app in the browser (redirects to docs page):**
```bash
scdev info                # check the URL and routing config
scdev services status     # ensure shared router is running
scdev services recreate   # rebuild router if config changed
```
If the app redirects to `docs.shared.scalecommerce.site`, the router can't reach your container.
Check that the container is running (`scdev status`) and that `routing.port` matches the port
your app actually listens on.

**File changes not reflected (macOS):**
```bash
scdev mutagen status      # check sync status
scdev mutagen reset       # recreate sync sessions
scdev mutagen flush       # wait for sync to complete
```

**Database connection refused:**
- Services talk by name. Use `db`, not `localhost`, in connection strings
- Example: `DATABASE_URL=postgres://postgres:postgres@db:5432/app`

**Email not showing up in Mailpit:**
- Ensure `shared.mail: true` in config
- Configure your app's SMTP to:
  - Host: `scdev_mail`, Port: `1025`, no auth, no TLS
  - Node.js (nodemailer): `{ host: 'scdev_mail', port: 1025, secure: false }`
  - PHP (Laravel): `MAIL_HOST=scdev_mail MAIL_PORT=1025 MAIL_ENCRYPTION=null`
  - Python (Django): `EMAIL_HOST='scdev_mail' EMAIL_PORT=1025`

## macOS / Mutagen Best Practices

On macOS, scdev uses Mutagen for file sync instead of Docker bind mounts. scdev's sync-ready
gate automatically holds the container's command until files are synced - no manual workarounds
needed. Just write a plain command like `pnpm dev`.

**Always add these to `mutagen.ignore` for Node.js/pnpm projects:**

```yaml
mutagen:
  ignore:
    - node_modules     # dependencies - stay in container, native speed
    - .pnpm-store      # pnpm's package store - platform-specific native binaries
    - .nuxt            # build cache
    - .output          # build output
```

**Why `.pnpm-store` matters:** pnpm creates a ~500MB content-addressable store inside the project
directory when running in a container. If this syncs to the host, it carries platform-specific
native binaries (glibc vs musl). When you switch images (e.g., `node:22` to `node:22-alpine`),
the wrong binaries get synced back into the new container, causing crashes like
`Error relocating ... fcntl64: symbol not found`.

**Performance:** With proper ignores, a Nuxt app with ~1000 dependencies installs in ~7s cold,
~2.5s warm. Without ignores, the same install takes 35s+ due to sync overhead.

### Mutagen Ignore by Stack

| Stack | Always ignore |
|-------|--------------|
| Node.js/pnpm | `node_modules`, `.pnpm-store`, `.nuxt`, `.next`, `.output` |
| Node.js/npm | `node_modules` |
| PHP/Composer | `vendor`, `var/cache`, `var/log` |
| Python | `__pycache__`, `.venv`, `*.pyc` |
| General | Build output dirs, cache dirs, log dirs |

## Config Templates by Stack

### Node.js (Nuxt/Next/Vite)

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
    - .output
```

### PHP + MySQL (Laravel/Symfony/Shopware)

```yaml
name: my-shop

services:
  app:
    image: webdevops/php-nginx:8.3
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app
    environment:
      WEB_DOCUMENT_ROOT: /app/public
      DATABASE_URL: mysql://root:root@db:3306/app
    routing:
      port: 80

  db:
    image: mysql:8.0
    volumes:
      - db_data:/var/lib/mysql
    environment:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: app

mutagen:
  ignore:
    - vendor
    - var/cache
    - var/log
```

### Python + PostgreSQL (Django/FastAPI)

```yaml
name: my-api

services:
  app:
    image: python:3.12-slim
    command: pip install -r requirements.txt && python manage.py runserver 0.0.0.0:8000
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app
    environment:
      DATABASE_URL: postgres://postgres:postgres@db:5432/app
    routing:
      port: 8000

  db:
    image: postgres:16-alpine
    volumes:
      - db_data:/var/lib/postgresql/data
    environment:
      POSTGRES_PASSWORD: postgres

mutagen:
  ignore:
    - __pycache__
    - .venv
    - "*.pyc"
```

## Tips

- `version` and `domain` are optional - scdev defaults sensibly
- `routing.protocol` defaults to `http` when `port` is set
- Service names in `scdev exec` match the keys in `.scdev/config.yaml`
- The project URL is always `https://{name}.scalecommerce.site` where `{name}` is from config
- `scdev exec` runs inside the container's `working_dir` by default
- Named volumes persist across `scdev stop`/`scdev start` but are removed with `scdev down -v`
- All shared services are accessible via HTTPS in the browser
