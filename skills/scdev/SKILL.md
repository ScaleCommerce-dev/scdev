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
scdev exec app npm test            # Run tests
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
    command: npm run dev
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app
    routing:
      port: 3000
```

This gives you: `https://my-app.scalecommerce.site` with HTTPS, isolated network, shared services.

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
domain: custom.scalecommerce.site  # optional, defaults to {name}.scalecommerce.site

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
    command: npm run dev
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app        # bind mount (auto-synced via Mutagen on macOS)
      - node_modules:/app/node_modules  # named volume (persists across restarts)
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
    - .nuxt
    - var/cache
```

### Routing

- `protocol: http` - HTTP/HTTPS via Traefik (default). Project gets `https://{name}.scalecommerce.site`
- `protocol: tcp` - Raw TCP passthrough. Requires `port` (container) and `host_port` (host)
- `protocol: udp` - UDP passthrough. Same as TCP

Services within a project reach each other by container name: `db`, `redis`, `app`, etc.

### Named Volumes

Named volumes are auto-discovered from service volume mounts. No separate declaration needed.
A volume like `db_data:/var/lib/postgresql/data` is automatically created and managed.
`scdev down -v` removes all project volumes.

## Custom Commands (Justfiles)

Projects can define custom commands in `.scdev/commands/`:

```
.scdev/commands/
  setup.just       # scdev setup
  test.just        # scdev test
  deploy.just      # scdev deploy
```

Example `.scdev/commands/setup.just`:

```just
default:
    scdev exec app npm ci
    scdev exec app npm run db:migrate

seed:
    scdev exec app npm run db:seed
```

```bash
scdev setup          # runs default recipe
scdev setup seed     # runs 'seed' recipe
scdev test           # runs default recipe in test.just
```

Use this for: running tests, database migrations, seeding data, creating admin users,
building assets, or any project-specific workflow.

## Setting Up scdev for an Existing Project

### Step 1: Create the config

Create `.scdev/config.yaml` with the project's services. Use `${PROJECTPATH}:/app` for the
source code mount.

### Step 2: Add to .gitignore (optional)

```
# scdev local state
.scdev/local/
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

Run tests with `scdev exec app npm test` (or `scdev test` if a test.just exists).
```

## Debugging

**Container won't start:**
```bash
scdev logs -f <service>   # check what's failing
scdev down && scdev start # clean restart
scdev config              # verify resolved config looks right
```

**Can't reach the app in the browser:**
```bash
scdev info                # check the URL and routing config
scdev services status     # ensure shared router is running
scdev services recreate   # rebuild router if config changed
```

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
- Configure your app's SMTP to: host `scdev_mail`, port `1025`

## Tips

- Service names in `scdev exec` match the keys in `.scdev/config.yaml`
- The project URL is always `https://{name}.scalecommerce.site` where `{name}` is from config
- `scdev exec` runs inside the container's `working_dir` by default
- Named volumes persist across `scdev stop`/`scdev start` but are removed with `scdev down -v`
- On macOS, add build artifacts and dependency dirs to `mutagen.ignore` for better sync performance
- All shared services are accessible via HTTPS in the browser
