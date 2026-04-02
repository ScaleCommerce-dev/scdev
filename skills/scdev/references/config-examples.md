# Config Examples by Stack

## Node.js (Nuxt/Next/Vite)

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

## PHP + MySQL (Laravel/Symfony/Shopware)

```yaml
name: my-shop

variables:
  DB_PASSWORD: root
  DB_NAME: ${PROJECTNAME}

services:
  app:
    image: webdevops/php-nginx:8.3
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

mutagen:
  ignore:
    - vendor
    - var/cache
    - var/log
```

## Python + PostgreSQL (Django/FastAPI)

```yaml
name: my-api

variables:
  DB_PASSWORD: postgres

services:
  app:
    image: python:3.12-slim
    command: pip install -r requirements.txt && python manage.py runserver 0.0.0.0:8000
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app
    environment:
      DATABASE_URL: postgres://postgres:${DB_PASSWORD}@db:5432/app
    routing:
      port: 8000

  db:
    image: postgres:16-alpine
    volumes:
      - db_data:/var/lib/postgresql/data
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD}

mutagen:
  ignore:
    - __pycache__
    - .venv
    - "*.pyc"
```

## Mutagen Ignore by Stack

| Stack | Always ignore |
|-------|--------------|
| Node.js/pnpm | `node_modules`, `.pnpm-store`, `.nuxt`, `.next`, `.output` |
| Node.js/npm | `node_modules` |
| PHP/Composer | `vendor`, `var/cache`, `var/log` |
| Python | `__pycache__`, `.venv`, `*.pyc` |
| General | Build output dirs, cache dirs, log dirs |

## Email SMTP Config by Stack

Configure your app to use scdev's shared Mailpit:
- Host: `scdev_mail`, Port: `1025`, no auth, no TLS
- Node.js (nodemailer): `{ host: 'scdev_mail', port: 1025, secure: false }`
- PHP (Laravel): `MAIL_HOST=scdev_mail MAIL_PORT=1025 MAIL_ENCRYPTION=null`
- Python (Django): `EMAIL_HOST='scdev_mail' EMAIL_PORT=1025`
