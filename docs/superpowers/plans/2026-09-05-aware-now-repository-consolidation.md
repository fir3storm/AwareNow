# AwareNow Repository Consolidation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `fir3storm/AwareNow` the single complete repository for application source, web frontend, deployment, templates, scripts, and documentation.

**Architecture:** Use the existing AwareNow repository as the destination root. Import the outer repository's compatible assets into that root, remove nested Git metadata, then normalize paths, CI, documentation, and ignore rules around the unified layout.

**Tech Stack:** Go, React/Vite, TypeScript, Bash, Python, nginx, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-09-05-aware-now-repository-consolidation-design.md`

## Global Constraints

- The canonical remote is `https://github.com/fir3storm/AwareNow.git`.
- Do not modify `https://github.com/fir3storm/gophish.git`.
- Do not copy `.git` directories, runtime data, databases, certificates, private keys, dependency caches, or Python bytecode.
- Browser-visible product naming must use AwareNow.
- The unified repository must support root-level Go commands and `web/` frontend commands.

### Task 1: Inventory and collision map

**Files:**
- Read: `D:/My Softwares/Gophish` outer repository
- Read: `D:/My Softwares/Gophish/awarenow-source` destination repository
- Create: `docs/superpowers/plans/2026-09-05-aware-now-repository-consolidation-inventory.md`

- [ ] Record every outer top-level asset and classify it as application, deployment, template, script, documentation, generated, runtime, or secret-bearing.
- [ ] Compare destination and source paths and record collision decisions for `README.md`, `.gitignore`, CI, and any shared scripts.
- [ ] Confirm the destination branch is clean and record its current commit before migration.
- [ ] Commit the inventory so the migration has an auditable input set.

### Task 2: Import outer repository assets

**Files:**
- Create or copy: `deploy/`
- Create or copy: `templates/`
- Create or copy: `scripts/`
- Create or merge: `docs/`
- Modify: `README.md`
- Modify: `.gitignore`

- [ ] Copy only the approved outer assets into the AwareNow root; exclude `.git`, `.worktrees`, `.commandcode`, `runtime`, databases, certificates, keys, `__pycache__`, and dependency caches.
- [ ] Merge README content into one AwareNow-facing document that explains local development, deployment, domains, ports, template imports, and authorized-use constraints.
- [ ] Merge ignore rules so `runtime/`, databases, TLS material, dependency directories, and generated files remain untracked.
- [ ] Preserve the cleanup-safety implementation and its tests in the root `scripts/` tree.
- [ ] Commit the imported repository structure.

### Task 3: Normalize paths and product identity

**Files:**
- Modify: `go.mod`
- Modify: `scripts/install.sh`
- Modify: `scripts/import-templates.py`
- Modify: `scripts/keep-english-templates.py`
- Modify: deployment examples under `deploy/`
- Modify: browser-facing assets under `templates/` and `static/` or `web/`

- [ ] Change relative path resolution to use the unified repository root rather than the former outer/nested boundary.
- [ ] Ensure deployment commands, template import commands, and cleanup commands work from the AwareNow root.
- [ ] Remove browser-visible legacy product strings while retaining explicit compatibility identifiers needed by the upstream API or migration code.
- [ ] Add or retain focused tests for cleanup scope and API-key validation.
- [ ] Run `git diff --check` and commit the path/identity normalization.

### Task 4: Normalize frontend and CI

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `web/package.json`
- Modify: `web/package-lock.json` only if dependency metadata changes
- Modify: frontend source/configuration files that reference the old root

- [ ] Make the workflow run Go checks from the repository root and frontend checks from `web/`.
- [ ] Install frontend dependencies with `npm ci`, then run `npm run lint` and `npm run build`.
- [ ] Fix TypeScript type-only import errors exposed by the installed TypeScript version.
- [ ] Keep Go 1.21, 1.22, and 1.23 validation unless a documented compatibility reason requires a change.
- [ ] Commit CI and frontend fixes.

### Task 5: Full validation and publish

**Files:**
- Read: all tracked files in the unified AwareNow repository
- Modify: only files required by validation failures

- [ ] Clone or verify the unified tree from a clean Git state without nested repository metadata.
- [ ] Run `go test ./...`, `npm ci`, `npm run lint`, `npm run build`, cleanup tests, and deployment/script dry runs.
- [ ] Search tracked browser-facing assets and docs for unintended legacy product-name references.
- [ ] Confirm no secrets or runtime artifacts are tracked with `git ls-files` checks.
- [ ] Push the completed commits to `fir3storm/AwareNow`.
- [ ] Verify the remote branch matches the final local commit and report the old Gophish remote as intentionally unchanged.
