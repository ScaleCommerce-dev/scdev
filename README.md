# scdev

**Stop fighting your dev environment. Start coding.**

scdev is a local development environment framework that gets you from `git clone` to coding in seconds. No more "works on my machine", no more port conflicts, no more Docker Compose spaghetti.

```bash
cd my-project
scdev start
# That's it. Your project is running at https://my-project.scalecommerce.site
```

## Why scdev?

You're a developer, not a DevOps engineer. You want to write code, not debug Docker networking for the hundredth time.

| Problem | scdev Solution |
|---------|----------------|
| "It works on my machine" | Identical containerized environments for everyone |
| Port conflicts between projects | Shared router - every project gets its own subdomain |
| Slow file sync on macOS | Mutagen file sync - native speed |
| Each project reinvents the wheel | Shared services with battle-tested defaults |
| Complex Docker Compose files | Simple YAML config with sensible defaults |
| Days to onboard new developers | Clone, `scdev start`, done |
| Push → wait → CI fails → repeat | Run the same commands locally before you push |

### Shared Services = Less Work, More Consistency

Every project needs routing, email testing, and database management. Instead of configuring these for each project (and getting it slightly different each time), scdev provides them once for all your projects:

- **Router** - HTTPS routing with automatic certificates. No more port conflicts, no more SSL setup.
- **Mail catcher** - Every project sends mail to the same place. Test emails without configuration.
- **Database UI** - Browse any project's database instantly. Already connected, already working.
- **Redis UI** - Visual Redis browser for all your projects. Inspect keys, run queries, analyze memory.

This isn't just about saving time (though you will). It's about **consistency**. When every project uses the same infrastructure, you eliminate an entire category of "why does this work differently here?" problems. New team members see the same setup everywhere. Best practices are baked in, not documented and forgotten.

## Quick Start

### Installation

```bash
# Install scdev (macOS/Linux)
curl -fsSL https://raw.githubusercontent.com/ScaleCommerce-DEV/scdev/main/install.sh | sh

# First run - sets up SSL certificates and shared services
scdev systemcheck
```

### Your First Project

1. Create a project config:

```bash
mkdir -p my-app/.scdev
cat > my-app/.scdev/config.yaml << 'EOF'
name: my-app

services:
  app:
    image: node:20-alpine
    command: npm run dev
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app
    environment:
      PORT: "3000"
    routing:
      port: 3000
EOF
```

2. Start it:

```bash
cd my-app
scdev start
```

3. Open https://my-app.scalecommerce.site in your browser.

That's it. No reverse proxy config, no SSL setup, no port mapping. It just works.

## Features

### Automatic HTTPS

Every project gets HTTPS out of the box. scdev uses [mkcert](https://github.com/FiloSottile/mkcert) to generate locally-trusted certificates.

```
https://my-app.scalecommerce.site     # Your project
https://mail.shared.scalecommerce.site # Email catcher
https://db.shared.scalecommerce.site   # Database UI
```

### Shared Services

Why run a mail catcher for every project? scdev shares infrastructure:

| Service | URL | What it does |
|---------|-----|--------------|
| Router | `router.shared.scalecommerce.site` | Router dashboard |
| Mail | `mail.shared.scalecommerce.site` | Catches all emails (Mailpit) |
| DB UI | `db.shared.scalecommerce.site` | Database browser (Adminer) |
| Redis UI | `redis.shared.scalecommerce.site` | Redis browser (Redis Insights) |

### Fast File Sync (macOS)

Docker Desktop's file sharing is notoriously slow on macOS. scdev automatically uses [Mutagen](https://mutagen.io/) to sync files at native speed.

```yaml
# This "just works" on macOS - no extra config needed
volumes:
  - ${PROJECTPATH}:/app
```

### Custom Commands

Define project-specific commands using [just](https://github.com/casey/just):

```bash
# .scdev/commands/setup.just
default:
    scdev exec app npm install
    scdev exec app npm run build

migrate:
    scdev exec app npm run db:migrate
```

Then run them:

```bash
scdev setup          # Runs default recipe
scdev setup migrate  # Runs migrate recipe
```

## Configuration

### Project Config (`.scdev/config.yaml`)

```yaml
name: my-shopware-shop
domain: shop.scalecommerce.site  # Optional - defaults to {name}.scalecommerce.site

# Connect to shared services
shared:
  router: true          # Connect to shared router (default: true)
  mail: true            # Mailpit for emails
  db: true              # Adminer for database browsing
  redis_insights: true  # Redis Insights for Redis browsing

# Environment variables available to all services
environment:
  APP_ENV: dev
  APP_DEBUG: "true"

services:
  app:
    image: php:8.2-fpm
    working_dir: /var/www
    volumes:
      - ${PROJECTPATH}:/var/www
      - composer_cache:/root/.composer
    environment:
      DATABASE_URL: mysql://root:root@db:3306/shopware
    routing:
      port: 9000
      protocol: http

  db:
    image: mysql:8.0
    volumes:
      - db_data:/var/lib/mysql
    environment:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: shopware

  redis:
    image: redis:7-alpine

# Paths to exclude from Mutagen sync (macOS)
mutagen:
  ignore:
    - var/cache
    - var/log
    - "*.log"
```

### Global Config (`~/.scdev/global-config.yaml`)

Auto-created on first run. Usually you don't need to touch this.

```yaml
domain: scalecommerce.site

ssl:
  enabled: true

mutagen:
  enabled: auto  # auto = enabled on macOS, disabled on Linux
```

## Commands

### Lifecycle

```bash
scdev start       # Start the project
scdev stop        # Stop containers (keeps them for quick restart)
scdev restart     # Stop + start
scdev down        # Remove containers and network
scdev down -v     # Remove everything including volumes
```

### Development

```bash
scdev exec app bash              # Shell into a container
scdev exec app npm test          # Run a command
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
scdev mail               # Open Mailpit in browser
scdev db                 # Open Adminer in browser
scdev redis              # Open Redis Insights in browser
```

### Mutagen (macOS file sync)

```bash
scdev mutagen status  # Check sync status
scdev mutagen flush   # Wait for sync to complete
scdev mutagen reset   # Recreate sync sessions (if stuck)
```

## Examples

### PHP + MySQL (Shopware/Laravel/Symfony)

```yaml
name: my-shop

services:
  app:
    image: webdevops/php-nginx:8.2
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

```

### Node.js + PostgreSQL

```yaml
name: my-api

services:
  app:
    image: node:20-alpine
    command: npm run dev
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app
      - node_modules:/app/node_modules
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

```

### Static Site / Frontend

```yaml
name: my-docs

services:
  app:
    image: node:20-alpine
    command: npm run dev
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app
    routing:
      port: 5173  # Vite default
```

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                    YOUR BROWSER                             │
│         https://my-app.scalecommerce.site                   │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                 SHARED SERVICES                             │
│              (scdev_shared network)                         │
│                                                             │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌─────────────┐           │
│  │ Router │ │Mailpit │ │Adminer │ │Redis Insight│           │
│  │ :80    │ │ :8025  │ │ :8080  │ │   :5540     │           │
│  │ :443   │ │ :1025  │ │        │ │             │           │
│  └───┬────┘ └────────┘ └────────┘ └─────────────┘           │
└────────┼────────────────────────────────────────────────────┘
         │
         │  docker network connect
         ▼
┌─────────────────────────────────────────────────────────────┐
│               PROJECT: my-app                               │
│            (my-app.scdev network)                           │
│                                                             │
│   ┌─────────┐  ┌─────────┐  ┌─────────┐                     │
│   │   app   │  │   db    │  │  redis  │                     │
│   │  :3000  │  │  :3306  │  │  :6379  │                     │
│   └─────────┘  └─────────┘  └─────────┘                     │
│                                                             │
│   Services communicate via DNS: db, redis, app              │
└─────────────────────────────────────────────────────────────┘
```

Each project gets its own isolated network. The router directs traffic based on the hostname and connects to project networks as needed.

## Troubleshooting

### "DNS doesn't resolve"

The domain `scalecommerce.site` is a wildcard DNS that points to `127.0.0.1`. If it doesn't work:

1. Check your DNS: `dig my-app.scalecommerce.site`
2. Some corporate networks block external DNS - try on a different network
3. Add entries to `/etc/hosts` as a workaround

### "Containers won't start"

```bash
scdev down           # Clean up
scdev start          # Try again
scdev logs -f app    # Check what's happening
```

### "File sync is slow" (macOS)

Mutagen should be enabled automatically. Check with:

```bash
scdev mutagen status
```

If it's stuck:

```bash
scdev mutagen reset
```

### "Port already in use"

scdev uses ports 80 and 443 for the router. Make sure nothing else is using them:

```bash
lsof -i :80
lsof -i :443
```

## Requirements

- Docker Desktop (macOS/Windows) or Docker Engine (Linux)
- macOS, Linux, or Windows with WSL2

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup.

## License

MIT
