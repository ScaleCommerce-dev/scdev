# Creating scdev Templates

Templates let users scaffold new projects with `scdev create`. A template is a GitHub repository or local directory that contains an `.scdev/` configuration and optionally starter files.

```bash
scdev create myorg/my-template my-app   # Any GitHub repo
scdev create express my-app             # Shorthand for ScaleCommerce-DEV/scdev-template-express
scdev create ./local-dir my-app         # Local directory (for development/testing)
```

## What's in a template

Every template has the same base structure:

```
my-template/
  .scdev/
    config.yaml              # Container configuration (image, ports, volumes, etc.)
    commands/
      setup.just             # Setup script (install deps, scaffold project)
  README.md                  # Usage instructions
```

Beyond this, a template may or may not include app source files. This depends on the framework:

**Include source files** when there's no scaffolding command. Example: an Express template ships with `app.js` and `package.json` because Express has no `create` command - you just write files and install dependencies.

**Don't include source files** when the framework has its own scaffolding. Example: a Nuxt template ships only `.scdev/` because `nuxi init` generates all app files. A Symfony template ships only `.scdev/` because `symfony new` does the same. Including source files that the scaffolder also creates would cause conflicts.

## The setup lifecycle

After `scdev create`, the user's workflow is:

```bash
scdev create <template> my-app
cd my-app
scdev setup
```

`scdev setup` runs the `setup.just` file which handles everything: starting containers, installing dependencies, scaffolding the project (if needed), and signaling that the app is ready to run.

### Why setup.just is needed

Templates need a setup step because the container alone isn't enough. Dependencies must be installed, frameworks may need scaffolding, and the app needs to be configured before it can serve requests. Without a setup step, the container would start but have nothing to run.

The setup justfile runs on the **host** and uses `scdev exec` to run commands inside the container. This is important because `scdev exec` provides an interactive terminal - the user can respond to prompts from package managers and scaffolding tools. The container entrypoint has no terminal, so interactive prompts would crash there.

To learn how to write setup files, see [Writing setup.just](#writing-setupjust).

### Why .setup-complete is needed

There's a circular dependency between the container and setup:
- The container must be running for `scdev exec` to work (setup needs the container)
- But the app can't start until setup finishes (the container needs setup)

The `.setup-complete` marker file solves this. The container entrypoint checks for it:

```yaml
command: >-
  sh -c "
  if [ ! -f .setup-complete ]; then
    echo 'Waiting for setup... Run: scdev setup';
    while [ ! -f .setup-complete ]; do sleep 2; done;
  fi;
  pnpm install && exec pnpm dev"
```

1. **First start** - no `.setup-complete` exists. The container enters a wait loop, staying alive without crashing. Now `scdev exec` works.
2. **Setup runs** - installs deps, scaffolds if needed, then `touch .setup-complete`. The wait loop detects the marker and starts the app.
3. **On restart** - `.setup-complete` exists, so the container skips the wait loop, runs dependency install (to pick up any new packages), and starts the app immediately.

The dependency install command (`pnpm install`, `composer install`, etc.) must be in the entrypoint so that restarting the container after adding a new package works without re-running setup.

## Writing config.yaml

The config defines your containers, routing, and sync settings:

```yaml
version: 1
name: ${PROJECTDIR}
domain: ${PROJECTNAME}.${SCDEV_DOMAIN}

info: |
  ## ${PROJECTNAME}
  
  Description of the project.
  Run `scdev setup` to get started.

variables:                    # reusable values for ${VAR} substitution in this file
  DB_PASSWORD: root
  DB_NAME: ${PROJECTNAME}

shared:                       # connect shared services to project's docker network
  router: true
  mail: true                  # Mailpit - catch all outgoing email (SMTP at mail:1025)
  db: true                    # Adminer - browse databases via web UI
  redis: true                 # Redis Insights - browse Redis keys via web UI

environment:                  # env vars passed to ALL containers
  APP_ENV: dev

services:
  app:
    image: node:22-alpine
    command: >-
      sh -c "corepack enable &&
      if [ ! -f .setup-complete ]; then
        echo 'Waiting for setup... Run: scdev setup';
        while [ ! -f .setup-complete ]; do sleep 2; done;
      fi;
      pnpm install && exec pnpm dev"
    working_dir: /app
    volumes:
      - ${PROJECTPATH}:/app
    environment:                # env vars passed to THIS container only
      NODE_ENV: development
      COREPACK_ENABLE_DOWNLOAD_PROMPT: "0"
      HOST: "0.0.0.0"
      PORT: "3000"
      DATABASE_URL: mysql://root:${DB_PASSWORD}@db:3306/${DB_NAME}
    routing:
      port: 3000
      # domain: api.${PROJECTNAME}.${SCDEV_DOMAIN}  # optional: custom domain for this service

  db:
    image: mysql:8.0
    volumes:
      - db_data:/var/lib/mysql
    environment:
      MYSQL_ROOT_PASSWORD: ${DB_PASSWORD}
      MYSQL_DATABASE: ${DB_NAME}

mutagen:
  ignore:
    - node_modules
    - .pnpm-store
    - .scdev
    - .setup-complete
```

### Variables vs environment

**`variables`** define reusable `${VAR}` placeholders that are substituted throughout the config file. They are not passed to containers. Use them to avoid duplicating values like database passwords across services.

**`environment`** (project-level) defines environment variables passed to ALL containers. **`services.<name>.environment`** (service-level) defines environment variables for that specific container only, and overrides project-level env vars with the same name.

Built-in variables are always available: `${PROJECTDIR}`, `${PROJECTPATH}`, `${PROJECTNAME}`, `${SCDEV_DOMAIN}`, `${SCDEV_HOME}`, `${USER}`, `${HOME}`, plus all host environment variables. User-defined `variables` can reference built-in ones (e.g. `DB_NAME: ${PROJECTNAME}_db`).

**Dev server binding:** Dev servers typically listen on `localhost` by default, which isn't accessible from outside the container. Set `HOST=0.0.0.0` (or the framework's equivalent) in the environment so the dev server binds to all interfaces.

**Multi-service routing:** Projects with multiple HTTP services (frontend + backend) can give each service its own domain using `routing.domain`. Only available for HTTP/HTTPS routing. Without it, all services share the project domain.

### Volumes: bind mounts vs named volumes

Volumes in the `services.<name>.volumes` list come in two forms:

**Bind mounts** map a host directory into the container. Your source code goes here - edits on the host are reflected in the container immediately (via Mutagen on macOS, direct mount on Linux).

```yaml
volumes:
  - ${PROJECTPATH}:/app              # host directory -> container path
```

**Named volumes** are persistent storage managed by Docker. Use them for data that should survive container recreation but doesn't belong on the host - database files, dependency directories, caches.

```yaml
volumes:
  - ${PROJECTPATH}:/app              # bind mount: source code
  - db_data:/var/lib/mysql           # named volume: database files
  - node_modules:/app/node_modules   # named volume: dependencies (alternative to Mutagen ignore)
```

How to tell them apart: if the left side starts with `/`, `./`, `../`, or `${` it's a bind mount. Otherwise it's a named volume.

Named volumes persist across `scdev stop`/`scdev start` and `scdev down`. They are only removed with `scdev down -v`. No top-level declaration needed - scdev discovers them automatically from the volume entries.

**When to use named volumes vs Mutagen ignore for dependencies:** Both approaches keep dependencies inside the container. Named volumes are explicit and work everywhere. Mutagen ignore is simpler (no extra volume entry) but only applies when Mutagen is active (macOS). For templates, prefer Mutagen ignore for Node.js `node_modules` since it's the standard scdev pattern.

### Configuring Mutagen ignores

Docker bind mounts on macOS are notoriously slow - operations like `pnpm install` or `composer install` that touch thousands of files can take 5-10x longer than native. scdev solves this by using [Mutagen](https://mutagen.io/) for fast bidirectional file sync between your host and a Docker volume. This happens automatically on macOS (on Linux, bind mounts are already fast, so Mutagen is not used).

The Mutagen ignore list controls which paths are **not synced** in either direction. Ignored paths exist only inside the container's volume. This is essential for dependencies like `node_modules` or `vendor` - they contain platform-specific binaries that must match the container's OS, and syncing thousands of dependency files would negate Mutagen's performance gains.

Add directories that should stay inside the container and not sync to the host:

```yaml
mutagen:
  ignore:
    - node_modules       # Native modules, platform-specific (Node.js)
    - .pnpm-store        # pnpm content-addressable store
    - vendor             # Composer dependencies (PHP)
    - .scdev             # scdev config (only needed on host)
    - .setup-complete    # Marker file (must persist in container volume)
```

Add framework-specific build artifacts:
- Nuxt: `.nuxt`, `.output`
- Next.js: `.next`
- Symfony: `var` (cache, logs)

**Critical:** `.setup-complete` MUST be in the ignore list. Since it's ignored, it persists in the container's Mutagen volume independently of the host. If it were synced, it could be deleted on one side and propagate to the other, breaking the setup state.

## Commands (justfiles)

Templates can include commands in `.scdev/commands/`. Each `.just` file becomes a `scdev` subcommand:

```
.scdev/commands/
  setup.just     ->  scdev setup
  test.just      ->  scdev test
  seed.just      ->  scdev seed
```

Commands are written as [just](https://github.com/casey/just) recipes. Just is a command runner (think `make` without the build system baggage). A justfile can have multiple recipes, arguments, dependencies between recipes, conditional logic, and more. See the [just documentation](https://just.systems/man/en/) for the full syntax.

When you run `scdev <command>`, scdev looks for `.scdev/commands/<command>.just` and executes it. If the justfile has multiple recipes, you can run a specific one with `scdev <command> <recipe>`. If no recipe is given, just runs the `default` recipe (if defined). For example:

```just
# .scdev/commands/test.just

default: unit           # scdev test -> runs unit tests

unit:                   # scdev test unit
    scdev exec app pnpm test

watch:                  # scdev test watch
    scdev exec app pnpm test --watch

e2e:                    # scdev test e2e
    scdev exec app pnpm test:e2e
```

Templates typically include at least `setup.just`. You can add more commands for common tasks like running tests, seeding databases, or deploying. These commands are discoverable - `scdev --help` lists them, and agents can `ls .scdev/commands/` to find them.

Justfiles run on the **host**, not inside the container. Use `scdev exec app <command>` to run things inside the container.

## Writing setup.just

```just
# Description of what setup does

[no-exit-message]
default:
    scdev start
    scdev exec app sh -c "your install commands here && touch .setup-complete"
    @echo ""
    @echo "Setup complete! App will start automatically."
    @echo ""
    @echo "Here are the details about your new project:"
    @echo ""
    scdev info
```

**Conventions:**
- `scdev start` goes first - the container must be running before `scdev exec`
- `touch .setup-complete` goes last in the exec - only after everything succeeds
- Use `@` prefix on cosmetic echo lines to suppress just's command echo
- Keep echo ON for `scdev start`, `scdev exec`, and `scdev info` so the user sees what's running
- Add `[no-exit-message]` to suppress just's default exit message

## Handling framework scaffolding

Frameworks like Nuxt and Symfony have their own scaffolding commands (`nuxi init`, `symfony new`). These commands typically expect an empty directory, which conflicts with `.scdev/` already being there.

There are two approaches depending on whether the scaffolding tool supports a force flag.

### Scaffold in-place (when the tool supports --force)

If the scaffolding command can run in a non-empty directory, scaffold directly in `/app`. This is the cleanest approach - no copying, no path issues.

On macOS with Mutagen, `.scdev` is in the ignore list so the container sees an essentially empty `/app`. On Linux with bind mounts, `.scdev/` is visible but scaffolding tools just add their own files alongside it.

**Example - Nuxt** (`nuxi init` supports `--force`) - `.scdev/commands/setup.just`:

```just
[no-exit-message]
default:
    scdev start -q
    @echo ""
    @echo "Installing tools..."
    scdev exec app sh -c "corepack enable && apk add --no-cache git"
    @echo ""
    @echo "Scaffolding Nuxt project..."
    scdev exec app pnpm dlx nuxi@latest init . --packageManager pnpm --gitInit=false --force
    @echo ""
    @echo "Preparing Nuxt modules..."
    scdev exec app npx nuxi prepare
    @echo ""
    @echo "Approving native module builds..."
    scdev exec app pnpm approve-builds --all
    @echo ""
    @echo "Finalizing..."
    scdev exec app sh -c "echo '.setup-complete' >> .gitignore && touch .setup-complete"
    @echo ""
    @echo "Setup complete! App will start automatically."
    @echo ""
    scdev info
```

Key details:
- `nuxi init .` scaffolds into the current directory, not a temp dir
- `--force` allows a non-empty directory
- `npx nuxi prepare` runs Nuxt module initialization, which may prompt to install missing dependencies (e.g. `better-sqlite3` for `@nuxt/content`). This runs interactively via `scdev exec` so prompts work. Without this, the same prompts would fire in the container entrypoint where there's no terminal, crashing the container.
- `pnpm approve-builds --all` approves native module build scripts after prepare (which may have installed new packages)
- `echo '.setup-complete' >> .gitignore` appends our marker to the scaffolder's gitignore
- `COREPACK_ENABLE_DOWNLOAD_PROMPT` is set in config.yaml's environment, not repeated in each command

### Scaffold in /tmp (when the tool requires an empty directory)

Some scaffolding tools have no force flag and strictly require an empty directory. In this case, scaffold in `/tmp` inside the container, then copy the files to `/app`.

**This approach is safe for PHP** because Composer's autoloader uses `__DIR__` relative paths resolved at runtime. Moving `vendor/` between directories works fine. **It does NOT work reliably for Node.js/pnpm** because pnpm uses a symlink-based content-addressable store with paths tied to the install location.

**Example - Symfony** (`symfony new` requires an empty directory) - `.scdev/commands/setup.just`:

```just
[no-exit-message]
default:
    scdev start -q
    @echo ""
    @echo "Installing dependencies..."
    scdev exec app apk add --no-cache bash
    @echo ""
    @echo "Installing Composer..."
    scdev exec app sh -c "wget -qO- https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer"
    @echo ""
    @echo "Installing Symfony CLI..."
    scdev exec app sh -c "wget https://get.symfony.com/cli/installer -O - 2>/dev/null | bash && cp \$HOME/.symfony5/bin/symfony /usr/local/bin/symfony"
    @echo ""
    @echo "Scaffolding Symfony project..."
    scdev exec app symfony new /tmp/app --no-git
    @echo ""
    @echo "Copying project files..."
    scdev exec app sh -c "cp -r /tmp/app/. /app/ && rm -rf /tmp/app && echo '.setup-complete' >> .gitignore && touch .setup-complete"
    @echo ""
    @echo "Setup complete! App will start automatically."
    @echo ""
    scdev info
```

Key details:
- `symfony new /tmp/app --no-git` scaffolds into a temp directory (Symfony has no `--force` for existing dirs)
- `--no-git` skips git init, avoiding the need for git config in the container
- Composer and Symfony CLI are installed to `/usr/local/bin` so they're available in subsequent exec calls
- `cp -r /tmp/app/. /app/` copies all files (including dotfiles) into the project directory. The `.scdev/` directory in `/app` is preserved because Symfony doesn't create one.
- Composer and Symfony CLI are installed at runtime since `php:8.4-cli-alpine` doesn't include them

### When to use which approach

| Approach | Use when | Examples |
|----------|----------|---------|
| In-place with `--force` | The scaffolding tool supports non-empty directories | Nuxt (`nuxi init . --force`) |
| /tmp + copy | The tool strictly requires an empty directory AND the ecosystem's dependency dir is portable | Symfony (`symfony new`), Laravel |
| No scaffolding needed | The template includes all source files | Express, static sites |

**Rule of thumb:** Prefer in-place scaffolding when possible. Only use the /tmp approach when the tool has no force flag, and only when the language ecosystem supports moving the dependency directory (PHP/Composer: yes, Node.js/pnpm: no).

## Framework-specific notes

### Node.js with pnpm

**Corepack prompt:** Node.js ships with corepack but pnpm must be downloaded on first use. Suppress the confirmation prompt:

```sh
export COREPACK_ENABLE_DOWNLOAD_PROMPT=0 && corepack enable
```

Always `export` the variable so it applies to all subsequent commands in the same `sh -c`.

**Native module build scripts:** pnpm v10 blocks build scripts by default. After installing dependencies, run:

```sh
pnpm approve-builds --all
```

This approves all pending native modules (like `better-sqlite3`, `esbuild`, `@parcel/watcher`) non-interactively and triggers their build/download of prebuilt binaries. Run it AFTER `pnpm install` so there are packages to approve. It saves the approvals to `package.json` so they persist across reinstalls.

**File watching:** Use Node.js built-in `--watch` mode (Node 22+) for automatic restarts:

```json
"scripts": {
  "start": "node --watch app.js"
}
```

Frameworks like Nuxt and Next.js have their own HMR - no extra config needed.

**Runtime dependency prompts:** Some Nuxt modules (like `@nuxt/content`) prompt to install missing dependencies at runtime. These prompts need a terminal which the container entrypoint doesn't have. Fix: run the framework's prepare step during setup (when `scdev exec` provides a terminal). For Nuxt: `npx nuxi prepare`.

### PHP with Composer

**Installing Composer:** The `php:*-cli-alpine` images don't include Composer. Install it at runtime:

```sh
wget -qO- https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer
```

**Installing Symfony CLI:** For the Symfony dev server and `symfony new` command:

```sh
apk add --no-cache bash
wget https://get.symfony.com/cli/installer -O - 2>/dev/null | bash
export PATH="$HOME/.symfony5/bin:$PATH"
```

**Dev server:** Use the Symfony CLI server for full compatibility:

```sh
symfony server:start --no-tls --port=8000 --allow-all-ip
```

`--no-tls` because scdev handles HTTPS via Traefik. `--allow-all-ip` binds to `0.0.0.0`.

**vendor/ portability:** Unlike Node.js, PHP's `vendor/` directory uses `__DIR__` for path resolution at runtime. Copying `vendor/` between directories (e.g. from `/tmp/app` to `/app`) works fine. This is why the /tmp scaffolding approach is safe for PHP but not for Node.js.

### Don't include files that the scaffolder creates

For scaffold templates, don't include `.gitignore`, `README.md`, or any files that the scaffolding tool will create. The scaffolder's versions will take precedence. If you need scdev-specific entries (like `.setup-complete`), append them to the scaffolder's `.gitignore` after setup:

```sh
echo '.setup-complete' >> .gitignore
```

## Naming and publishing

Name your template repository `scdev-template-<name>`:

```
myorg/scdev-template-react    -> scdev create myorg/scdev-template-react my-app
myorg/scdev-template-django   -> scdev create myorg/scdev-template-django my-app
```

The `ScaleCommerce-DEV` org has a shorthand - bare names resolve to `ScaleCommerce-DEV/scdev-template-<name>`:

```
scdev create express   -> ScaleCommerce-DEV/scdev-template-express
scdev create nuxt4     -> ScaleCommerce-DEV/scdev-template-nuxt4
```

## Testing

During development, test your template locally by referencing the directory:

```bash
scdev create ./my-template test-app
cd test-app
scdev setup
```

Verify:
- `scdev setup` completes without errors
- The app URL (`https://test-app.scalecommerce.site`) loads correctly
- `scdev restart` works (entrypoint picks up dependencies)
- File changes are reflected (HMR or `--watch` mode)

## Existing templates

Browse all available templates on GitHub: [ScaleCommerce-DEV repositories matching `scdev-template-`](https://github.com/orgs/ScaleCommerce-DEV/repositories?q=scdev-template-). Each template's README explains what it includes and how to use it.

## Common pitfalls

**Container crashes before setup runs.**
The entrypoint must keep the container alive when `.setup-complete` doesn't exist. Use the wait loop pattern. Never use a command that can fail before setup (like `pnpm start` unconditionally).

**`scdev exec` fails with "service not running".**
The container crashed. Check `scdev logs` to see why. Common causes: the entrypoint command failed, missing `.setup-complete` wait loop, or a syntax error in the shell command.

**Native modules fail with "Ignored build scripts".**
pnpm v10 blocks build scripts by default. Run `pnpm approve-builds --all` after `pnpm install` to approve and rebuild them.

**Framework prompts crash with "TTY initialization failed".**
Some frameworks prompt interactively at runtime (e.g. `@nuxt/content` asking to install `better-sqlite3`). These prompts need a terminal which the container entrypoint doesn't have. Fix: trigger these checks during setup (when `scdev exec` provides a terminal) by running the framework's prepare step. For Nuxt: `npx nuxi prepare`.

**Corepack asks "Do you want to continue?"**
Set `export COREPACK_ENABLE_DOWNLOAD_PROMPT=0` before running `corepack enable`. Must be `export` so it applies to subsequent commands in the same `sh -c`.

**Dev server not accessible via browser.**
The dev server is listening on `localhost` inside the container. Set `HOST=0.0.0.0` in the environment (Node.js) or use `--allow-all-ip` (Symfony CLI) so it binds to all interfaces.

**Scaffolding tool complains about non-empty directory.**
If the tool supports a force flag (`--force`), use it to scaffold in-place. If not (like `symfony new`), scaffold in `/tmp` and copy back. See "Handling framework scaffolding" above.

**Changes on host not reflected in container (or vice versa).**
Check the Mutagen ignore list. Ignored paths are not synced in either direction. Dependency directories (`node_modules`, `vendor`) should be ignored (they stay in the container). Source files should NOT be ignored.
