# AwareNow Repository Consolidation Design

## Goal

Make `fir3storm/AwareNow` the single complete repository for the AwareNow application, deployment, templates, scripts, and documentation.

## Current problem

The outer `fir3storm/gophish` repository contains deployment assets, templates, and operational scripts, while `awarenow-source` is an independent Git repository containing the Go backend and web frontend. The outer repository currently embeds that repository as a nested directory, creating two remotes and making a checkout incomplete unless the nested repository is also initialized.

## Target architecture

The existing AwareNow repository becomes the canonical root. Its top level will contain the existing Go application and web frontend plus the outer repository's compatible `deploy/`, `templates/`, `scripts/`, and `docs/` directories. `awarenow-source` will no longer be a nested Git repository. The public product name and repository identity will be AwareNow; compatibility names may remain only where required by upstream APIs or migration behavior.

## Migration rules

1. Preserve the AwareNow application history and remote: `https://github.com/fir3storm/AwareNow.git`.
2. Copy outer deployment, template, script, README, license, and design assets into the AwareNow root, resolving collisions explicitly.
3. Do not copy runtime data, databases, certificates, private keys, generated caches, or nested `.git` metadata.
4. Update all scripts, Go module references, frontend paths, CI paths, README commands, and deployment examples to work from the AwareNow root.
5. Keep the Gophish compatibility name only where it is required for upstream API compatibility or an explicit migration note; remove it from browser-visible UI and the canonical product-facing documentation.
6. Preserve the completed cleanup-safety commit and keep unfinished hardening work isolated until its tasks are implemented and tested.

## Validation criteria

- A fresh clone of `fir3storm/AwareNow` contains backend, frontend, templates, deployment files, scripts, and docs without a nested repository.
- `go test ./...` runs from the repository root after dependencies are resolved.
- `npm ci`, `npm run lint`, and `npm run build` run from `web/`.
- Template import and cleanup scripts resolve paths relative to the unified root.
- CI references the unified root and continues Go-version coverage.
- `git diff --check` passes and no secrets/runtime artifacts are tracked.
- The old `fir3storm/gophish` remote is not modified by this migration.
