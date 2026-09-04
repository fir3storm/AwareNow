# AwareNow Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Secure deployment and cleanup while removing legacy product naming from the browser UI.

**Spec:** `docs/superpowers/specs/2026-09-04-awarenow-hardening-design.md`

## Global Constraints

- User-visible UI content must use `AwareNow` and contain no `Gophish` string.
- Cleanup can delete only explicitly identified retired HailBytes records.
- TLS terminates at nginx; backend listens on localhost.
- CI retains Go 1.21, 1.22, and 1.23.

### Task 1: Safe cleanup

Modify `scripts/keep-english-templates.py`; create `scripts/tests/test_keep_english_templates.py`.

- [ ] Prove a user-created non-English record is retained and missing `--gophish` credentials prevent local removal.
- [ ] Run the test and verify it fails.
- [ ] Validate credentials before mutation; select only `name in drop_names`.
- [ ] Re-run the focused test and verify it passes.

### Task 2: TLS-safe installation and cookies

Modify `scripts/install.sh`, `deploy/nginx-gophish.conf.example`, `deploy/config.json.example`, and the necessary `awarenow-source` configuration, route, session, and Go tests.

- [ ] Add failing tests for `cookie_secure` on CSRF and session cookies.
- [ ] Implement the option, safe nginx upgrade behavior, HTTPS redirect, and forwarded protocol header.
- [ ] Run focused tests and nginx configuration validation.

### Task 3: Browser UI rebranding

Modify legacy templates, browser JS source, and generated assets in `awarenow-source`; create `awarenow-source/scripts/check-ui-branding.sh`.

- [ ] Add a failing audit for browser-served legacy-name text.
- [ ] Replace visible names, titles, tooltips, documentation links, and browser storage keys with AwareNow equivalents; rebuild assets.
- [ ] Re-run audit and verify no matches.

### Task 4: Frontend CI verification

Modify `awarenow-source/.github/workflows/ci.yml`.

- [ ] Add Node 22 `npm ci`, lint, and build job for `web`.
- [ ] Run frontend CI commands plus final cleanup, Go, formatting, branding, and diff checks.
