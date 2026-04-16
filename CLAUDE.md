# scdev

Local development environment framework for web applications. Go CLI that shells out to the `docker` CLI (never the Docker SDK - keeps the door open for Podman). Single command startup, shared infrastructure (Traefik, Mailpit, Adminer), project isolation via Docker networks.

**About this file:** agent guidance only - decisions, conventions, and gotchas that can't be inferred from code. Every line biases behavior and competes for attention. When editing, ask per line: "Would removing this cause an agent to make a mistake?" If not, cut it. Don't add file listings, stack summaries, or anything agents discover by grepping.

## Workflow

- Run `make test` before every commit (fast, mock runtime).
- Run `make test-integration` before releases, or after changing project lifecycle / routing / Mutagen / runtime code (spins up real Docker, takes minutes).
- **Never commit, push, or tag without explicit user confirmation.** Show the proposed commit message and wait for approval before `git commit`, `git push`, or `git tag`. Never add "Co-Authored-By" lines.

Release process:
1. Add `## vX.Y.Z` section at the top of `CHANGELOG.md`.
2. `git add CHANGELOG.md && git commit -m "Release vX.Y.Z"`
3. `git tag vX.Y.Z && git push origin main && git push origin vX.Y.Z`
4. CI builds darwin/linux (arm64/amd64) binaries and creates the GitHub Release from the changelog.

## Style

**Never use em-dashes** (—). Use regular hyphens (-) everywhere: code, copy, comments, docs.

## Conventions That Break Expectations

- **No top-level `volumes:` in project config.** Unlike Docker Compose, named volumes don't need separate declaration - anything in a service `volumes:` entry that doesn't start with `/` or `.` is auto-discovered as a named volume (`parseVolumeMount()`).
- **Config `variables:` are NOT env vars.** They're `${VAR}` placeholders substituted at config-load time (second pass of `LoadProject()`, after `PROJECTNAME` resolves). They don't reach containers - that's what `environment:` is for.
- **Justfile commands** live in `.scdev/commands/<name>.just`, not a single Justfile. Resolution order: built-in > justfile > error.
- **Mutagen auto-detection:** enabled on macOS, disabled on Linux. Controlled by `~/.scdev/global-config.yaml`, not project config.
- **`routing.domain`** on a service enables a per-service custom domain (HTTP/HTTPS only). Without it, services share the project domain. Useful for frontend + backend splits.
- **Default domain `scalecommerce.site`** is wildcard DNS resolving to 127.0.0.1 - not a real site, just a resolver trick.
- **Framework progress messages use `ui.StatusStep()`**, not `fmt.Println` - two blank lines + cyan `▶` + bold text, so they stand out from verbose nested command output. Mirrored by the `scdev step <message>` subcommand for template justfiles (templates should call `@scdev step "..."` instead of `@echo "..."` for top-level progress markers).

## Architecture Anchors

- **`internal/config/defaults.go`** is the single source of truth for images, versions, and the default domain. Change once, everything picks it up.
- **`buildContainerConfig()` in `internal/project/project.go`** is the single source of truth for container configuration. It stamps an `scdev.config-hash` label covering image, env, volumes, command, working dir, routing labels, ports, and network aliases. `scdev update` recreates any service whose stamped hash differs. **Any new service config field that should shape a container must flow through `buildContainerConfig` - otherwise `scdev update` won't detect changes to it.**
- **`ContainerNameFor(service, project)`** builds container names without a loaded `Project`. Use it instead of `fmt.Sprintf("%s.%s.scdev", ...)`.
- **Link networks** are runtime relationships between projects, stored in global state (`~/.scdev/state.yaml`), not project config. Each creates a `scdev_link_<name>` network. Containers resolve each other by container name via Docker's embedded DNS.
- **Template repos** follow the naming convention `scdev-template-<name>` (matters for `scdev create` resolution).

## Adding Docker-Dependent Commands

Call `requireDocker(ctx)` as the first line of `RunE` (defined in `cmd/shared.go`). Without it users get cryptic low-level failures instead of a clear "Docker isn't running" message.

### Adding a Shared Service

Easy-to-miss steps when wiring a new shared service:

1. Container name constant in `internal/services/<service>.go`.
2. `Start<Service>` / `Stop<Service>` / `<Service>Status` on `manager.go`.
3. `Connect<Service>ToProject` **must pass network aliases** so project containers can resolve it by short name.
4. Update `runServicesRecreate()` in `cmd/services.go` - stop/remove/start.
5. Wire into `cmd/services.go` start/stop/status commands.
6. Add the image constant to `internal/config/defaults.go`.

## Gotchas

- **Project domains don't work for inter-container communication.** `*.scalecommerce.site` resolves to 127.0.0.1, which inside a container points at the container itself, not Traefik. Cross-project containers must use container names (`app.project-b.scdev`) - this is why `scdev link` uses Docker DNS, not routing.
- **Mutagen ignored paths are not synced either way.** `node_modules` and `.pnpm-store` must be ignored for Node.js projects so they stay inside the container at native speed. IDE autocomplete still works via the host's own `pnpm install` / host `node_modules`.
- **`.pnpm-store` MUST be in the mutagen ignore list for pnpm projects.** pnpm builds a ~500MB content-addressable store with platform-specific native binaries (glibc vs musl) inside the project dir. Without ignoring it, syncing those binaries to the host breaks the next time the container image changes.
- **Only directory bind mounts sync via Mutagen.** Single-file mounts stay as regular bind mounts.
- **Sync-ready gate:** `buildContainerConfig` wraps commands with a wait on `/.scdev-sync-ready` when Mutagen is enabled for that service. Don't add your own `while [ ! -f ... ]` workaround - it's already there.
- **`scdev rename` migrates volumes via a temp container** using a project service image (guaranteed present locally). Docker has no native volume rename. See `CopyVolume` on the Runtime interface. All copies happen before any old volumes are removed, to bound blast radius on failure.
- **The docs page (`docs.shared.<domain>`) is also Traefik's 404 catch-all** - unmatched URLs land there, not a generic error page.
- **Integration tests that tear down shared services** (router, mail, db, redis) must snapshot beforehand and restore afterward via `snapshotSharedServices` / `restoreSharedServices`. Forgetting this silently breaks the developer's running environment.

## Docs to Keep in Sync

- **`README.md`** - user-facing docs and marketing; needs updating for any user-visible change (new commands, config options, shared services, CLI flags).
- **`templates/README.md`** - template authoring guide; update alongside `README.md` when changing config options, variables, Mutagen behavior, or the create/setup workflow.
- **`CONTRIBUTING.md`** - developer onboarding (structure, testing strategy, how to add commands/services). Update it when architectural decisions or test patterns change.
- **`Completo-Briefing.md`** - context for Completo's AI features. Regenerate with `/completo-briefing`.
