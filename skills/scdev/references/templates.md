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

Setup also produces a wall of output (apk/composer/npm installing hundreds of deps, compilers,
scaffolders). Plain `@echo` status lines disappear in that noise. `@scdev step "<msg>"` prints two
leading blank lines + a cyan `▶` + bold text so each phase reads as a clear section header. Styling
auto-strips when stdout isn't a TTY, when `NO_COLOR` is set, or when global plain mode is on, so
the same recipe works in logs and CI.

```just
# Description

[no-exit-message]
default:
    scdev start -q
    @scdev step "Installing dependencies"
    scdev exec app sh -c "corepack enable && pnpm install && touch .setup-complete"
    @scdev step "Setup complete! App will start automatically."
    scdev info
```

Conventions:
- `scdev start -q` first (quiet: skips info display since setup shows it at the end)
- `touch .setup-complete` last in exec (only on success)
- `@scdev step "<msg>"` for each top-level phase; reserve `@echo` for sub-detail lines that don't need to stand out
- Keep echo ON for `scdev start`, `scdev exec`, `scdev info` so the user sees what's running
- `[no-exit-message]` suppresses just's exit message
- Break long chains into separate `scdev exec` calls with a `@scdev step` marker between them

## Forwarding colon-namespaced CLIs

For wrappers like `bin/console cache:clear` or `artisan migrate:fresh`, declare a recipe
named after the file. scdev auto-prepends it so colon args pass through as recipe params
instead of being parsed as just's module path:

```just
# .scdev/commands/console.just
console *args:
    scdev exec app php bin/console {{args}}
```

`scdev console cache:clear` -> `bin/console cache:clear`. Without a filename-matching
recipe, the first arg is still treated as the recipe name (legacy behavior).

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

## Stack-specific runtime gotchas

Language/framework behaviors (Node corepack, pnpm build scripts, PHP extensions,
`SYMFONY_TRUSTED_PROXIES`, Webpack Encore asset pipelines, `memory_limit`, `MAILER_DSN`) are **not
template-authoring-specific** — they apply to any scdev-managed container. See
`stack-gotchas.md` for the full list. When authoring a template you'll bake those into the
entrypoint and/or `setup.just`; when adding scdev to an existing project you'll bake them into the
entrypoint the same way.

## Template Naming and Testing

Name repos `scdev-template-<name>`. The `ScaleCommerce-DEV` org has shorthand:
`scdev create express` -> `ScaleCommerce-DEV/scdev-template-express`

Test locally: `scdev create ./my-template test-app && cd test-app && scdev setup`

Verify: setup completes, URL loads, `scdev restart` works, file changes reflected.
