# Creating scdev Templates

Templates let users scaffold new projects with `scdev create`. A template is a GitHub repo
(or local directory) containing `.scdev/` configuration and optionally starter files.

## Template Structure

```
my-template/
  .scdev/
    config.yaml              # Container config, routing, volumes, mutagen
    commands/
      setup.just             # Setup script
  README.md
```

**Static templates** (e.g. Express) also include source files (`app.js`, `package.json`, `.gitignore`).
**Scaffold templates** (e.g. Nuxt, Symfony) ship only `.scdev/` - the framework's create command generates app files during setup.

## The .setup-complete Pattern

Templates must solve a circular dependency: the container needs to be running for `scdev exec`
(used by setup), but the app can't start until setup completes.

**In config.yaml command:**

```yaml
command: >-
  sh -c "corepack enable &&
  if [ ! -f .setup-complete ]; then
    echo 'Waiting for setup... Run: scdev setup';
    while [ ! -f .setup-complete ]; do sleep 2; done;
  fi;
  pnpm install && exec pnpm dev"
```

- No marker: container enters wait loop (stays alive for `scdev exec`)
- Setup creates marker: loop exits, app starts
- On restart: marker exists, skips loop, runs dep install + app

The dep install (`pnpm install`, `composer install`) MUST be in the entrypoint so restart
picks up new packages without re-running setup.

**In setup.just** - `touch .setup-complete` goes last, only after everything succeeds.

**In mutagen.ignore** - `.setup-complete` MUST be ignored so it persists in the container
volume independently of the host.

## Writing setup.just

Setup runs on the **host**, uses `scdev exec` for container commands. The interactive terminal
from `scdev exec` is important - framework prompts work here but would crash in the entrypoint
(which has no TTY).

```just
# Description

[no-exit-message]
default:
    scdev start -q
    @echo ""
    @echo "Installing dependencies..."
    scdev exec app sh -c "corepack enable && pnpm install && touch .setup-complete"
    @echo ""
    @echo "Setup complete! App will start automatically."
    @echo ""
    @echo "Here are the details about your new project:"
    @echo ""
    scdev info
```

Conventions:
- `scdev start -q` first (quiet: skips info display since setup shows it at the end)
- `touch .setup-complete` last in exec (only on success)
- `@echo` for cosmetic lines (suppresses just's command echo)
- Keep echo ON for `scdev start`, `scdev exec`, `scdev info` so the user sees what's running
- `[no-exit-message]` suppresses just's exit message
- Break long chains into separate `scdev exec` calls with `@echo` progress messages between them

## Handling Framework Scaffolding

When the framework has a create command that expects an empty directory:

### Scaffold in-place (when tool supports --force)

On macOS with Mutagen, `.scdev` is ignored so the container sees an empty `/app`. On Linux,
`.scdev/` is visible but scaffolding tools just add their own files alongside it.

Example - Nuxt:

```just
scdev exec app pnpm dlx nuxi@latest init . --packageManager pnpm --gitInit=false --force
scdev exec app npx nuxi prepare          # triggers module dep prompts interactively
scdev exec app pnpm approve-builds --all  # approves native module build scripts
scdev exec app sh -c "echo '.setup-complete' >> .gitignore && touch .setup-complete"
```

`npx nuxi prepare` is critical - it runs Nuxt module initialization which may prompt for
missing dependencies (e.g. `better-sqlite3` for `@nuxt/content`). Without it, those prompts
fire in the entrypoint where there's no TTY, crashing the container.

### Scaffold in /tmp (when tool requires empty directory)

Copy files back after scaffolding. Safe for PHP (Composer uses `__DIR__` relative paths) but
NOT for Node.js/pnpm (symlink-based store with absolute paths).

Example - Symfony:

```just
scdev exec app symfony new /tmp/app --no-git
scdev exec app sh -c "cp -r /tmp/app/. /app/ && rm -rf /tmp/app"
scdev exec app sh -c "echo '.setup-complete' >> .gitignore && touch .setup-complete"
```

### When to use which

| Approach | Use when | Examples |
|----------|----------|---------|
| In-place with `--force` | Tool supports non-empty dirs | Nuxt (`nuxi init . --force`) |
| /tmp + copy | Tool requires empty dir AND deps are portable | Symfony, Laravel |
| No scaffolding | Template includes all source files | Express, static sites |

## Node.js/pnpm Specifics

- Set `COREPACK_ENABLE_DOWNLOAD_PROMPT: "0"` in config.yaml environment (not in each command)
- `pnpm approve-builds --all` after install to approve native module prebuilt binaries
- `HOST: "0.0.0.0"` in environment so dev server is accessible from outside container
- Add to mutagen ignore: `node_modules`, `.pnpm-store`, `.scdev`, `.setup-complete`
- Framework build artifacts: `.nuxt`, `.output` (Nuxt), `.next` (Next.js)
- Use `node --watch app.js` for file watching (Node 22+); frameworks have their own HMR

## PHP/Composer Specifics

- `php:8.4-cli-alpine` doesn't include Composer or Symfony CLI - install at runtime
- Install Composer: `wget -qO- https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer`
- Install Symfony CLI: `wget https://get.symfony.com/cli/installer -O - 2>/dev/null | bash && cp $HOME/.symfony5/bin/symfony /usr/local/bin/symfony`
- Install tools to `/usr/local/bin` so they're available in subsequent `scdev exec` calls
- Symfony dev server: `symfony server:start --no-tls --port=8000 --allow-all-ip`
- `--no-tls` because scdev handles HTTPS via Traefik; `--allow-all-ip` binds to 0.0.0.0
- Add to mutagen ignore: `vendor`, `var`, `.scdev`, `.setup-complete`

## Template Naming and Testing

Name repos `scdev-template-<name>`. The `ScaleCommerce-DEV` org has shorthand:
`scdev create express` -> `ScaleCommerce-DEV/scdev-template-express`

Test locally: `scdev create ./my-template test-app && cd test-app && scdev setup`

Verify: setup completes, URL loads, `scdev restart` works, file changes reflected.
