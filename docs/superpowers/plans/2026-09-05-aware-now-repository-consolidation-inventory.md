# AwareNow Repository Consolidation Inventory

Date: 2026-09-05
Task: 1 — inventory and collision map
Source checkout: `D:/My Softwares/Gophish` (`fir3storm/gophish`)
Destination checkout: `D:/My Softwares/Gophish/awarenow-source` (`fir3storm/AwareNow`)

## Snapshot and repository state

| Checkout | Branch | HEAD | Remote | Tracked files | Working tree at inventory time |
|---|---|---|---|---:|---|
| Outer source | `main` | `43f02a2267be20195726f2b3d02d20514b4aaf93` | `https://github.com/fir3storm/gophish.git` | 289 | Existing untracked files/directories |
| Destination | `main` | `342136518cc38227287aeecb477c92c94f0b0c71` | `https://github.com/fir3storm/AwareNow.git` | 554 | Existing untracked files/directories |

The destination is not clean. Pre-existing untracked paths are `.superpowers/` and `docs/superpowers/plans/2026-09-05-aware-now-deployment-path-audit.md`. They are outside this Task 1 change and must not be deleted, overwritten, or staged accidentally. The outer checkout also has untracked `.commandcode/`, `awarenow-source/`, `docs/`, and `scripts/__pycache__/`; these are inventory inputs or local state, not migration inputs by default.

## Outer repository asset classification

The outer repository’s tracked top-level assets are classified as follows:

| Path | Count | Classification | Migration disposition |
|---|---:|---|---|
| `.gitattributes` | 1 | Repository metadata | Merge with destination rules; preserve LF rules for shell/service/example/JSON files and reconcile existing Go/static vendored rules. |
| `.gitignore` | 1 | Repository metadata and safety policy | Merge with destination ignore rules; retain runtime, database, TLS, key, binary, dependency-cache, and generated-file exclusions. |
| `LICENSE` | 1 | Legal/documentation | Collision; review attribution and select a single AwareNow-compatible license document. |
| `README.md` | 1 | Documentation/operations | Collision; merge deployment, domains, ports, import commands, and authorized-use language into an AwareNow-facing root README. |
| `deploy/` | 4 | Deployment configuration | Import as a new root directory, then normalize Gophish paths/names and review examples for AwareNow. Files: `config.json.example`, `gophish.service`, `nginx-gophish.conf.example`, `nginx-got-phished.snippet`. |
| `scripts/` | 5 | Operational scripts | Import as a new root directory. Review all path assumptions and preserve the cleanup-safety implementation/tests from the hardening work. Files: `enable-got-phished.py`, `enable-got-phished.sh`, `import-templates.py`, `install.sh`, `keep-english-templates.py`. |
| `templates/` | 276 | Application/template content and vendor assets | Merge into the destination `templates/` directory; destination currently has 15 server UI templates, while the outer tree has 17 INSEC assets and 258 vendor assets plus its README. No exact file-path collisions were found, but the directory is semantically shared. |
| `docs/` | 0 tracked, 2 on disk | Planning/design documentation | Review selectively. The two untracked outer files are the 2026-09-04 hardening design and plan; do not bulk-copy local planning state. Destination already contains the 2026-09-05 consolidation design and plan. |

Template breakdown in the outer tracked tree: `templates/insec/` contains 17 files, and `templates/vendor/` contains 258 files (HailBytes, LinkSec, and piyush27pawar material). `templates/README.md` is included in the 276-file count.

## Destination inventory relevant to the merge

The destination already provides the application and frontend foundation:

| Path/group | Count | Classification |
|---|---:|---|
| Go application packages and root files | 214+ | Application/backend, database migrations, configuration, tests, and build metadata |
| `web/` | 46 | React/Vite frontend source, configuration, lockfile, and public assets |
| `static/` | 278 | Existing server frontend/static assets; includes legacy-named assets that require a later identity audit |
| `templates/` | 15 | Existing server-rendered UI templates |
| `.github/` | 2 | Existing `ci.yml` and `release.yml`; CI is a file-level collision risk only if outer CI is later discovered/imported |
| `docs/` | 2 | Consolidation design and implementation plan already tracked at destination HEAD |
| `ansible-playbook/`, `docker/`, `Dockerfile` | 11 | Existing deployment/build assets; must be reconciled with imported `deploy/` rather than duplicated blindly |
| `package.json`, `gulpfile.js`, `webpack.config.js`, `yarn.lock` | 4 | Existing legacy frontend/build tooling; must coexist intentionally with `web/package.json` and `web/package-lock.json` |

## Exact collision map

Comparison of normalized tracked paths (`/` separators) found exactly four file collisions:

| Path | Outer SHA-256 | Destination SHA-256 | Decision |
|---|---|---|---|
| `.gitattributes` | `61F3EBA2F17355FFFEC44E83A44C2A4A4FD0E80771F783965D29AA8E9A21E4DE` | `E426B21E1B42A702EC366B8173B12864D9684297AFD3527EAEA74EDC69796048` | Merge; retain both repository line-ending policies and avoid losing destination Go/static vendored attributes. |
| `.gitignore` | `BA1CA600E5CADC4B9B9D0D1214587C7650F4C83A6A624B1AFD3E3C3F220D21A0C1` | `69A70711CA5801699619B7CD992E24B8B17474AD9BB3EDC121F1DADD66D5566B` | Merge; destination currently ignores core build artifacts, while outer rules add runtime/TLS safety. |
| `LICENSE` | `0A8932772020AA0B32EA636812305AEC0F3789B771D5DFFFDB42F03BBAD0E705` | `CC58D92CA666E6174CC3BD8C78F73D2D7B2E7B765651189E3D5F8AFD5A7970F5` | Legal review required before selecting/merging one canonical document. |
| `README.md` | `F3A05F510AA88D228D42B0FC9157C6151E44DE48D1563A7543AA811E5EF68179` | `535515218A69EBFB4A448FED81C26F9975A900F39BB7BDF262C9EBC3880A34ED` | Rewrite/merge into one AwareNow product-facing README; update clone URLs, commands, names, and deployment paths. |

Directory-level collisions/additions:

- `templates/`: outer operational/content templates and destination server UI templates share the directory name but have no exact tracked file-path collision. Preserve both sets and document their roles.
- `docs/`: destination has the consolidation documents; outer has separate untracked hardening documents. Merge only intentionally selected documentation.
- `deploy/`: absent from destination; add after reconciling existing `ansible-playbook/`, `docker/`, and `Dockerfile` behavior.
- `scripts/`: absent from destination; add after checking overlap with existing root tooling and `web/` scripts.
- `.github/`: absent from the outer tracked tree, so destination workflows remain authoritative unless later task analysis identifies path changes.

## Explicit exclusions

The following paths are not migration inputs:

| Path/pattern | Classification | Reason |
|---|---|---|
| Any `.git/` directory, including `awarenow-source/.git/` | Git metadata | The nested repository boundary must not be copied into the unified tree. Preserve history through Git operations, not nested metadata. |
| `.worktrees/` | Local worktree state | Isolated worktrees and their contents are local execution state. |
| `.code-review-graph/graph.db` | Generated/runtime database | Knowledge-graph output, not source. |
| `.commandcode/` | Local agent/tool state | Workspace metadata, not product source. |
| `.superpowers/` | Local agent/tool state | Destination execution state; currently untracked. |
| `**/__pycache__/` and `*.pyc` | Generated | Python bytecode must not be committed or imported. Existing examples occur under outer `scripts/` and vendor tools. |
| `web/node_modules/` and any dependency cache | Generated/dependency cache | Recreate from lockfiles with `npm ci`; never copy installed modules. |
| Runtime directories and files such as `runtime/`, `*.db`, `*.sqlite*` | Runtime/database | Campaign state and local databases must remain outside source control. |
| TLS/private material such as `*.crt`, `*.key`, `*.pem`, `*.p12`, `*.pfx` | Secret-bearing | Never migrate certificates, private keys, or credentials. |
| `static/db/geolite2-city.mmdb` already tracked by destination | Binary data/database-like asset | Existing destination artifact needs a separate provenance/licensing/tracking decision; do not copy or replace it from the outer tree as part of Task 1. |

The outer tree also contains untracked `awarenow-source/` itself. It is the destination checkout, not an asset to copy into itself.

## Migration risks and handoff notes

1. The outer README is production-specific to `itsupport.insec.in`, `/opt/gophish`, and the Gophish remote. A blind copy would publish stale repository identity, paths, ports, and product naming.
2. The destination README and much of `static/` retain Gophish branding. Browser-visible identity work must be handled in the planned normalization task, while API/database compatibility names may need to remain.
3. Imported `deploy/` files use Gophish filenames and service/path conventions. They must be reconciled with destination Ansible/Docker assets before deployment claims are made.
4. The outer template pack contains phishing simulation content and vendor licenses/readmes. Retain provenance and authorized-use constraints; review licensing before redistribution.
5. The destination has both legacy root build tooling and a separate `web/` Vite app. CI and developer commands must clearly select the intended frontend path.
6. The destination working tree is already dirty with unrelated untracked files. Later tasks must stage by explicit path and avoid deleting or absorbing those files.
7. The old `fir3storm/gophish` remote is explicitly out of scope for modification. The migration should publish only to `fir3storm/AwareNow`.

## Task 1 acceptance record

- [x] Outer top-level tracked assets classified.
- [x] Destination structure and relevant existing assets recorded.
- [x] Exact file collisions and directory-level collisions recorded.
- [x] Runtime, secret-bearing, generated, nested Git, and local-agent paths explicitly excluded.
- [x] Outer and destination HEAD/remotes/status captured.
- [ ] Application code copied or modified — intentionally not performed in Task 1.
