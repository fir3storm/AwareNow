# AwareNow Frontend and CI Readiness Audit

Date: 2026-09-05
Scope: frontend/CI readiness for the repository consolidation plan. This report is diagnostic only; no source files, workflow files, or package metadata were modified.

## Executive findings

1. `web/` is already the frontend project root. Its scripts are suitable for the unified layout: `npm run lint` invokes `oxlint`, and `npm run build` invokes `tsc -b && vite build`.
2. The current CI workflow runs only the Go build/format/test matrix. It has no `npm ci`, lint, or frontend build step.
3. The current frontend build fails with five `TS1484` diagnostics because `web/tsconfig.app.json` enables `verbatimModuleSyntax` and four UI components import React types as regular imports.
4. `npm ci --ignore-scripts` succeeds and `npm run lint` succeeds from `web/`; the failure is limited to the TypeScript build diagnostics observed below.

## Unified-root layout requirements

The consolidation spec makes `fir3storm/AwareNow` the single repository root. The expected layout is:

```text
AwareNow/
  <Go module and backend packages>
  deploy/
  docs/
  scripts/
  templates/
  web/
  .github/workflows/
```

Frontend implications:

- Keep the frontend under `web/`; do not move `web/src`, `web/public`, or its Vite/TypeScript configuration to the repository root.
- Run all frontend commands with `working-directory: web` (or an equivalent `cd web`), because `web/package.json` and `web/package-lock.json` are the frontend package boundary.
- Use `npm ci`, not `npm install`, in CI so the committed lockfile is authoritative.
- The current `vite.config.ts` has no former nested-repository path references. Its API proxy targets `http://localhost:3333`, which is appropriate for the local backend and does not need a consolidation path change.
- The current frontend browser-facing labels and storage keys found under `web/src` use `AwareNow`/`awarenow`; no Gophish branding was found in the frontend source/configuration scan.
- The root-level consolidation must retain the existing `web` directory while importing outer-repository `deploy/`, `templates/`, `scripts/`, and compatible `docs/` assets beside it. No frontend path should point to `awarenow-source/` after import.

## CI changes required

File: `.github/workflows/ci.yml`

The existing Go matrix (Go 1.21, 1.22, and 1.23) already checks out at the unified repository root and runs root-level `go get`, `go build`, formatting, and `go test`. Preserve that matrix and add a frontend job, or an equivalent independent step, with this sequence:

```yaml
frontend:
  name: Frontend
  runs-on: ubuntu-latest
  defaults:
    run:
      working-directory: web
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-node@v4
      with:
        node-version: <repository-supported-LTS>
        cache: npm
        cache-dependency-path: web/package-lock.json
    - run: npm ci
    - run: npm run lint
    - run: npm run build
```

The exact Node LTS value should be selected consistently with the repository’s supported runtime policy when Task 4 is implemented. The important path requirements are `working-directory: web` and `cache-dependency-path: web/package-lock.json`. If `defaults.run.working-directory` is used, keep checkout and setup-node at job scope; only shell commands need the frontend directory.

The workflow should not run `npm` from the repository root: the root currently has no package manifest, while `web/package.json` is the only frontend manifest. The current CI also has no dependency-cache setup for Node.

Related observation: `.github/workflows/release.yml` still uses legacy Gophish release names and packages legacy `static/*` paths. It is not part of this frontend/CI audit’s requested modification scope, but the consolidation task should review it before publishing because its assumptions do not match the AwareNow `web/` build layout.

## TypeScript build errors

Configuration: `web/tsconfig.app.json` line 13 enables `verbatimModuleSyntax: true`. Under this setting, symbols used only as types must be imported with `import type`.

Required source-only fixes:

| File | Current import | Required change |
| --- | --- | --- |
| `web/src/components/ui/Button.tsx:1` | `import { ButtonHTMLAttributes, ReactNode } from 'react';` | Split into `import type { ButtonHTMLAttributes, ReactNode } from 'react';` and retain the `Loader2` value import separately. |
| `web/src/components/ui/Card.tsx:1` | `import { ReactNode } from 'react';` | Change to `import type { ReactNode } from 'react';`. |
| `web/src/components/ui/Modal.tsx:1` | `import { ReactNode, useEffect } from 'react';` | Split the type import from the `useEffect` value import. |
| `web/src/components/ui/Table.tsx:1` | `import { ReactNode } from 'react';` | Change to `import type { ReactNode } from 'react';`. |

Observed diagnostics:

```text
Button.tsx(1,10): TS1484 ButtonHTMLAttributes is a type and must be imported using a type-only import.
Button.tsx(1,32): TS1484 ReactNode is a type and must be imported using a type-only import.
Card.tsx(1,10): TS1484 ReactNode is a type and must be imported using a type-only import.
Modal.tsx(1,10): TS1484 ReactNode is a type and must be imported using a type-only import.
Table.tsx(1,10): TS1484 ReactNode is a type and must be imported using a type-only import.
```

No other TypeScript diagnostics were emitted because `tsc -b` stops on these errors. The planned fix should preserve `verbatimModuleSyntax`; weakening that compiler option would hide import-boundary errors rather than align the source with the existing strict configuration.

## Verification observations

Commands run from `D:\My Softwares\Gophish\awarenow-source\web`:

| Command | Result |
| --- | --- |
| `npm ci --ignore-scripts` | Passed; 115 packages added, audit reported 0 vulnerabilities. |
| `npm run lint` | Passed; `oxlint` exited successfully with no diagnostics. |
| `npm run build` | Failed at `tsc -b` with the five `TS1484` diagnostics listed above; Vite did not run. |

## Recommended implementation order for Task 4

1. Apply the four type-only import corrections listed above.
2. Add the frontend CI job using `web` as its working directory and the committed lockfile for `npm ci`.
3. Run `npm ci`, `npm run lint`, and `npm run build` from `web/`.
4. Run `git diff --check` and confirm no root-level frontend path references `awarenow-source/`.

