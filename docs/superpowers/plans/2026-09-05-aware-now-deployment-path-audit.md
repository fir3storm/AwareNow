# AwareNow Deployment Path Audit

**Scope:** Outer repository deployment assets and operational scripts, audited before importing them into the existing AwareNow repository root.

**Inputs:**

- `docs/superpowers/specs/2026-09-05-aware-now-repository-consolidation-design.md`
- `docs/superpowers/plans/2026-09-05-aware-now-repository-consolidation.md`
- Outer repository `D:/My Softwares/Gophish/scripts/` and `D:/My Softwares/Gophish/deploy/`

## Findings

### 1. Repository-relative paths are already compatible after import

The three path-sensitive scripts use `Path(__file__).resolve().parents[1]` or
`dirname "$0"/..`:

- `scripts/import-templates.py`: `ROOT` resolves to the repository root and reads `ROOT/templates`.
- `scripts/keep-english-templates.py`: `ROOT` resolves to the repository root and reads `ROOT/templates/vendor/hailbytes`.
- `scripts/enable-got-phished.sh`: `REPO_ROOT` resolves to the repository root and invokes `REPO_ROOT/scripts/enable-got-phished.py`.
- `scripts/install.sh`: `REPO_ROOT` resolves to the repository root and reads `REPO_ROOT/deploy/*`.

When copied from the outer repository into the unified root, these expressions
continue to resolve correctly. No extra `awarenow-source/` component should be
introduced, and none of these expressions should be changed to `../awarenow-source`.

The outer campaign assets should be merged as children of the existing
application `templates/` directory:

```text
outer templates/insec/                 -> AwareNow templates/insec/
outer templates/vendor/                -> AwareNow templates/vendor/
outer scripts/*.py,*.sh                -> AwareNow scripts/
outer deploy/*                         -> AwareNow deploy/
```

The application UI templates already present under `AwareNow/templates/` are a
different namespace. Do not replace that directory wholesale; copy/merge the
`insec/` and `vendor/` subdirectories.

### 2. Hard-coded runtime root must be standardized

The current operational install layout is `/opt/gophish`, and the nginx aliases
assume campaign assets are at `/opt/gophish/templates`. For a canonical AwareNow
deployment, use `/opt/awarenow` as the installation root:

| Current reference | Required unified reference |
| --- | --- |
| `/opt/gophish/runtime` | `/opt/awarenow/runtime` |
| `/opt/gophish/templates/insec/static/got-phished.html` | `/opt/awarenow/templates/insec/static/got-phished.html` |
| `/opt/gophish/scripts/import-templates.py` | `/opt/awarenow/scripts/import-templates.py` |
| `/opt/gophish/scripts/enable-got-phished.sh` | `/opt/awarenow/scripts/enable-got-phished.sh` |
| `RUNTIME_DIR=/opt/gophish/runtime` | `RUNTIME_DIR=/opt/awarenow/runtime` |

Apply these changes in:

- `scripts/install.sh` lines 2-3, 9, and all user-facing install output that names the product or install location.
- `scripts/import-templates.py` lines 6-7 and 317 (usage and API-key lookup examples).
- `scripts/enable-got-phished.py` lines 15-16 (default `GOPHISH_GOT_PHISHED_HTML`) and line 127's stale “git pull the gophish repo” error text.
- `deploy/nginx-gophish.conf.example` line 38 (nginx `alias`).
- `deploy/nginx-got-phished.snippet` lines 3 and 7 (command and nginx `alias`).
- `deploy/gophish.service` lines 10-11 (`WorkingDirectory` and `ExecStart`).

The executable, database filename, system user, systemd unit, nginx site name,
and API environment variable names may remain `gophish` for this migration:
they are upstream/runtime compatibility identifiers, not browser-visible product
branding. If the implementation renames the unit or user, update every matching
`systemctl`, `journalctl`, ownership, and nginx symlink reference atomically.

### 3. Systemd and nginx deployment filenames are not repository path problems

The following destination paths are host configuration paths and should remain
outside the repository:

- `/etc/systemd/system/gophish.service`
- `/etc/nginx/sites-available/gophish`
- `/etc/nginx/sites-enabled/gophish`
- `/etc/nginx/snippets/gophish-got-phished.conf`

Their source files should be read from the unified root via
`$REPO_ROOT/deploy/...`; `scripts/install.sh` already does this correctly.
Do not rewrite these `/etc/...` destinations to repository-relative paths.

### 4. Template and static path assumptions

The imported `templates/insec/static/got-phished.html` is the only file-backed
campaign asset referenced by nginx. It must exist at the exact unified-root
deployment path above. The import scripts expect these additional paths:

```text
templates/insec/emails/*.html
templates/insec/landing/*.html
templates/vendor/hailbytes/
templates/vendor/linksec/emails/
templates/vendor/piyush27pawar/Email_Templates/
templates/vendor/piyush27pawar/Landing_Pages/
```

These paths are valid after a child-directory merge. Preserve case-sensitive
directory names (`Email_Templates`, `Landing_Pages`) because the scripts use
those exact names on Linux.

### 5. Existing hard-coded non-path assumptions to resolve during the same edit

These are not nested-repository path failures, but they will make the unified
deployment misleading or broken if left unchanged:

- `scripts/install.sh` describes an “AwareCheck VPS” and prints “Gophish is installed”; update product-facing text to AwareNow while retaining upstream release URLs and compatibility identifiers.
- `deploy/nginx-gophish.conf.example` describes “Gophish” and “AwareCheck”; update comments to AwareNow.
- `deploy/gophish.service` `Description=` can say `AwareNow campaign service (Gophish-compatible)`.
- `scripts/import-templates.py` and `scripts/keep-english-templates.py` can retain `GOPHISH_*` environment names and API terminology because those are compatibility interfaces; update only prose that presents Gophish as the product name.
- `deploy/config.json.example` contains relative runtime filenames (`gophish.db`, certificate names, and `db/db_`). These are resolved relative to the service `WorkingDirectory`, so they remain valid when the runtime directory changes to `/opt/awarenow/runtime`.

## Required implementation checklist

- [ ] Merge `templates/insec` and `templates/vendor` into the existing AwareNow `templates/`; do not overwrite application UI templates.
- [ ] Copy `scripts/` and `deploy/` to the AwareNow root, excluding `scripts/__pycache__/`.
- [ ] Keep `REPO_ROOT`/`ROOT` relative-resolution logic unchanged unless tests demonstrate a failure from an alternate current working directory.
- [ ] Change the default host runtime root from `/opt/gophish` to `/opt/awarenow` consistently across install, service, nginx, and helper-script examples.
- [ ] Decide explicitly whether the migration supports an existing `/opt/gophish` installation; if yes, add a documented compatibility/migration fallback rather than silently abandoning its database.
- [ ] Preserve `gophish` only for upstream release/API/runtime compatibility; remove it from browser-visible and canonical product-facing prose.
- [ ] Run shell syntax checks and Python compile/import checks without generating tracked bytecode; run script dry-runs with a temporary fixture or mocked API.

## Validation commands

From the unified AwareNow root:

```bash
bash -n scripts/install.sh scripts/enable-got-phished.sh
python3 -m py_compile -q scripts/import-templates.py scripts/keep-english-templates.py scripts/enable-got-phished.py
git diff --check
```

For path validation, execute from outside the repository and verify that the
scripts still discover the unified root:

```bash
cd /tmp
python3 /opt/awarenow/scripts/import-templates.py --help
python3 /opt/awarenow/scripts/keep-english-templates.py --dry-run
```

The import script has no CLI dry-run flag and requires an API key, so its path
behavior should be tested with a local mocked API or an explicitly authorized
staging Gophish-compatible service. Do not run cleanup against production as a
path test.

