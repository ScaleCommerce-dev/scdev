# scdev

Local development environment framework for web applications. Single command startup, shared infrastructure (Traefik, Mailpit, Adminer), project isolation via Docker networks.

**About this file:** CLAUDE.md is for agent guidance - architectural decisions, rules, conventions, and gotchas that can't be inferred from reading code. Don't bloat it with code-level details (file listings, prop docs, full API specs) that agents can discover by reading the source. Focus on the "why", not the "what".

## Quick Reference

```bash
make build           # Build binary
make test            # Run unit tests (always run before committing)
make test-integration # Run integration tests (requires Docker, run before releases or after refactors)
```

**When to run which:** `make test` is fast and should run on every change. `make test-integration` spins up real Docker containers and takes minutes - run it before releases, after refactoring core logic (project lifecycle, routing, Mutagen), or when changing code that unit tests can't cover.

## Releases

1. Update `CHANGELOG.md` - add new `## vX.Y.Z` section at top
2. Commit, tag, push:
   ```bash
   git add CHANGELOG.md && git commit -m "Release vX.Y.Z"
   git tag vX.Y.Z && git push origin main && git push origin vX.Y.Z
   ```
3. CI builds binaries for darwin/linux (arm64/amd64), creates GitHub Release with changelog

## Key Decisions

- **Runtime:** Shell out to `docker` CLI (not SDK) - enables future Podman support
- **Defaults:** All in `internal/config/defaults.go` - single source of truth for domain, images, versions
- **Templates:** Embedded via `//go:embed` with `${VAR}` substitution
- **Tests:** Unit tests from start, integration tests tagged `//go:build integration`
- **Default Domain:** `scalecommerce.site` - wildcard DNS resolving to 127.0.0.1
- **Named Volumes:** Auto-discovered from service volume mounts - no top-level `volumes:` section needed (unlike Docker Compose). `parseVolumeMount()` detects named vs bind volumes.
- **Config variables:** `variables:` in project config defines reusable `${VAR}` placeholders. Substituted in the second pass of `LoadProject()` after PROJECTNAME is resolved. NOT passed to containers (that's what `environment:` is for).
- **Project templates:** `scdev create` scaffolds from GitHub repos or local dirs. Template logic in `internal/create/`, command in `cmd/create.go`. Template repos follow the naming convention `scdev-template-<name>`.

## Style

- **Never use em-dashes** (—). Use regular hyphens (-) in all code, copy, comments, and docs.

## Conventions That Break Expectations

- **No top-level `volumes:` in project config.** Unlike Docker Compose, named volumes don't need separate declaration. They're discovered automatically from service `volumes:` entries. If it doesn't start with `/` or `.`, it's a named volume.
- **Justfile commands** live in `.scdev/commands/<name>.just`, not a single Justfile. Command resolution: built-in > justfile > error.
- **Mutagen auto-detection:** Enabled on macOS, disabled on Linux. Controlled by `~/.scdev/global-config.yaml`, not project config.
- **`routing.domain`** on services allows per-service custom domains (HTTP/HTTPS only). Without it, all services share the project domain. Useful for frontend + backend setups.

## Adding New Commands

All Docker-dependent commands must call `requireDocker(ctx)` at the top of their `RunE` function (defined in `cmd/shared.go`). This gives users a clear error if Docker isn't running instead of a confusing low-level failure.

## Adding New Shared Services

When adding a new shared service (easy to miss steps):

1. Add container name constant in `internal/services/<service>.go`
2. Add `Start<Service>`, `Stop<Service>`, `<Service>Status` methods to `manager.go`
3. **Update `cmd/services.go` `runServicesRecreate()`** - add stop/remove/start calls
4. Update `cmd/services.go` start/stop/status commands
5. Add image constant to `internal/config/defaults.go`

## Gotchas

- Mutagen ignored paths are NOT synced in either direction. `node_modules` and `.pnpm-store` should always be ignored for Node.js projects - they stay inside the container for native speed. IDE autocomplete still works because `pnpm install` also runs on the host (or the IDE uses the host's own node_modules).
- For pnpm projects, `.pnpm-store` MUST be in the mutagen ignore list. pnpm creates a ~500MB content-addressable store inside the project dir when running in a container. Without ignoring it, platform-specific native binaries (glibc vs musl) sync to the host and break when the container image changes.
- Only directory bind mounts are synced via Mutagen. Single-file mounts stay as regular bind mounts.
- The docs page (`docs.shared.<domain>`) doubles as a 404 catch-all via Traefik - unmatched URLs redirect there.
- The sync-ready gate (`/.scdev-sync-ready` marker) automatically holds the container's command until Mutagen sync completes. No need for `while [ ! -f ... ]` workarounds in commands.

## README

The README doubles as the project's main documentation and marketing page. It contains config examples, command references, and architecture explanations that must stay in sync with the code. Check README.md for needed updates on every user-facing change (new commands, config options, shared services, CLI flags).

## Templates Docs

`templates/README.md` is the template authoring guide. It documents config.yaml options, the setup lifecycle, scaffolding patterns, and framework-specific notes. When changing config options, variables, Mutagen behavior, or the create/setup workflow, update both `README.md` and `templates/README.md`.

## Contributing Guide

`CONTRIBUTING.md` is the developer onboarding doc - project structure, testing strategy, architecture decisions, and how to add commands/services. Update it when adding new packages, changing test patterns, or making architectural decisions that affect how developers work on the project.

## Completo Briefing

`Completo-Briefing.md` provides project context to Completo's AI features. Use `/completo-briefing` to regenerate.

