# AwareNow Multi-Tenant Control Plane Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first safe, multi-tenant AwareNow control-plane increment around the existing Go/Gophish-compatible campaign engine.

**Architecture:** A TypeScript/Express service manages tenant identity, safe event ingestion, auditing, provisioning state, and aggregate analytics in PostgreSQL. The Go engine adds a private, bearer-authenticated control surface that returns only safe campaign summaries. The existing React UI gains a tenant-aware control-plane dashboard shell, while deployment templates document one engine/database/network per tenant.

**Tech Stack:** Go 1.21, existing Go/Gophish-compatible engine, TypeScript, Express, Prisma/PostgreSQL, React 19, Vite, Vitest, oxlint, Docker Compose, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-09-05-awarenow-multitenant-control-plane-design.md`

## Global Constraints

- Work only on branch `feature/awarenow-control-plane`; do not push, deploy, or run a campaign.
- AwareNow is the only product name; do not add runtime imports from the PhishSentinel source tree.
- Every tenant must model a separate engine, database, worker identity, delivery credential reference, and domain routing.
- The control plane must reject credential values and credential-like event fields; store field names/count only.
- Tenant scope comes from authenticated `TenantPrincipal` data, never caller-supplied tenant IDs.
- Private engine endpoints require `AWARENOW_CONTROL_TOKEN` and expose no recipient address, template body, credential value, or form value.
- Tests must be deterministic and must not require network access, databases, Docker, outbound mail, or actual secrets.
- Preserve existing Go engine behavior outside the private control API.

---

## File structure and ownership

| Area | Owner | Files |
| --- | --- | --- |
| Baseline quality | Swarm 1 | `web/src/components/ui/*.tsx`, `.github/workflows/ci.yml`, `docs/development.md` |
| Architecture contract | Swarm 2 | `docs/architecture/control-plane-contracts.md` |
| Management data model | Swarm 3 | `control-plane/prisma/schema.prisma`, `control-plane/prisma/seed.ts` |
| Safety policy | Swarm 4 | `control-plane/src/security/*` |
| Engine boundary | Swarm 5 | `controllers/control/*` |
| Isolated deployment | Swarm 6 | `deploy/tenant-engine/*` |
| Tenant/event domain | Swarm 7 | `control-plane/src/tenancy/*`, `control-plane/src/events/*` |
| Dashboard building blocks | Swarm 8 | `web/src/control-plane/*` |
| Assembly and verification | Integration engineer | `control-plane/package.json`, `control-plane/tsconfig.json`, `control-plane/src/{app,server,config,engine,provisioning,analytics}/*`, root documentation and CI reconciliation |

## Task 1: Stabilize existing UI tooling and CI

**Files:**
- Modify: `web/src/components/ui/Button.tsx`, `Card.tsx`, `Modal.tsx`, `Table.tsx`
- Modify: `.github/workflows/ci.yml`
- Create: `docs/development.md`

**Interfaces:**
- Consumes: existing `web/package.json` scripts `lint` and `build`.
- Produces: a frontend CI job executing `npm ci`, `npm run lint`, and `npm run build` in `web/`.

- [ ] **Step 1: Reproduce the TypeScript compilation failure**

Run: `npm run build` from `web/`.
Expected: the initial baseline reports TS1484 errors for React type imports.

- [ ] **Step 2: Correct type-only imports**

Use `import type { ButtonHTMLAttributes, ReactNode } from 'react'` in `Button.tsx` and `import type { ReactNode } from 'react'` in the other three components. Keep runtime imports unchanged.

- [ ] **Step 3: Add isolated frontend CI job**

Add a `frontend` job on Ubuntu with checkout, Node 24 setup, `working-directory: web`, then run `npm ci`, `npm run lint`, and `npm run build` as distinct steps.

- [ ] **Step 4: Document safe local validation**

Create `docs/development.md` with the exact commands `cd web; npm ci; npm run lint; npm run build` and `go test ./...`; describe the known legacy SQLite/goose incompatibility only if still reproduced, without suggesting credential or campaign setup.

- [ ] **Step 5: Verify and commit**

Run `npm run lint`, `npm run build`, and `git diff --check`; commit with `fix: stabilize frontend build and CI`.

## Task 2: Publish the inter-service contract

**Files:**
- Create: `docs/architecture/control-plane-contracts.md`

**Interfaces:**
- Consumes: safe event envelope and endpoint requirements in the Spec.
- Produces: canonical TypeScript definitions and HTTP response examples for Tasks 5, 7, and 9.

- [ ] **Step 1: Write contract acceptance examples**

Document one valid `SafeEngineEvent`, one rejected payload containing `password`, `GET /api/v1/control/health` response, and one safe campaign summary response.

- [ ] **Step 2: Define exact contracts**

Define `TenantPrincipal`, `SafeEngineEvent`, `EngineHealth`, `SafeCampaignSummary`, `TenantLifecycle`, and `ProvisioningRequest` with field names identical to the Spec.

- [ ] **Step 3: State trust boundaries**

Specify that only the authenticated principal supplies tenant scope, event values are forbidden, engine tokens are redacted, and engine control routes are private.

- [ ] **Step 4: Validate and commit**

Run `git diff --check`; commit with `docs: define control plane contracts`.

## Task 3: Model the management-plane data

**Files:**
- Create: `control-plane/prisma/schema.prisma`
- Create: `control-plane/prisma/seed.ts`

**Interfaces:**
- Consumes: `TenantLifecycle`, safe event and engine concepts from Task 2.
- Produces: Prisma models `Tenant`, `TenantMembership`, `EngineInstance`, `SafeEngineEventRecord`, `MetricAggregate`, and `AuditEntry`.

- [ ] **Step 1: Write schema invariants as comments adjacent to models**

Document that engine URL, database reference, worker identity reference, delivery credential reference, and domain route are per tenant; document that `SafeEngineEventRecord` has no credential-value column.

- [ ] **Step 2: Create Prisma schema**

Use PostgreSQL datasource, `prisma-client-js` generator, UUID IDs, tenant lifecycle enum `PROVISIONING | ACTIVE | SUSPENDED | FAILED`, platform/tenant membership roles, unique `Tenant.slug`, unique `(tenantId, userId)` membership, unique engine `tenantId`, and unique safe event `(tenantId, externalEventId)`.

- [ ] **Step 3: Add deterministic seed input**

Provide a seed script that defines, but does not execute external provisioning for, `awarenow-demo` in `PROVISIONING` state and one engine record with reference strings only.

- [ ] **Step 4: Validate and commit**

Run `git diff --check`; commit with `feat: add control plane tenant schema`.

## Task 4: Implement safe event and secret policy primitives

**Files:**
- Create: `control-plane/src/security/safeEvent.ts`
- Create: `control-plane/src/security/redactSecrets.ts`
- Create: `control-plane/src/security/safeEvent.test.ts`
- Create: `control-plane/src/security/redactSecrets.test.ts`

**Interfaces:**
- Consumes: `SafeEngineEvent` fields from Task 2.
- Produces: `parseSafeEngineEvent(input: unknown, authenticatedTenantId: string): SafeEngineEvent` and `redactSecrets(value: unknown): unknown`.

- [ ] **Step 1: Write failing safe-event tests**

Cover one valid event, a tenant mismatch, `password` key, `credential` key, unknown key, invalid type, and an invalid ISO timestamp.

- [ ] **Step 2: Write failing redaction tests**

Cover nested `token`, `apiKey`, `secret`, and `authorization` keys; assert that their values are `[REDACTED]` while normal keys remain unchanged.

- [ ] **Step 3: Implement the minimal policy**

Reject unknown event keys and case-insensitive forbidden keys `password`, `secret`, `token`, `credential`, and `value`. Permit only the event types in the Spec and field metadata as `fieldNames`/`fieldCount`.

- [ ] **Step 4: Run focused tests and commit**

Run the two test files using the eventual control-plane test command if available, otherwise record the command required by the integration task; commit with `feat: enforce safe event data policy`.

## Task 5: Add the Go engine private control boundary

**Files:**
- Create: `controllers/control/control.go`
- Create: `controllers/control/control_test.go`

**Interfaces:**
- Consumes: `AWARENOW_CONTROL_TOKEN`, `EngineHealth`, and `SafeCampaignSummary` from Task 2.
- Produces: `NewHandler(token string, campaignReader CampaignReader, campaignStopper CampaignStopper) http.Handler` with `GET /health`, `GET /campaigns`, and `POST /campaigns/{id}/stop`.

- [ ] **Step 1: Write failing handler tests**

Cover missing bearer token (401), wrong bearer token (401), health response (200), safe campaign list response (200), and stop request for a non-numeric campaign ID (400).

- [ ] **Step 2: Define narrow engine interfaces**

Define `CampaignReader.ListSafeCampaigns() ([]SafeCampaignSummary, error)` and `CampaignStopper.StopCampaign(id int64) error` so no legacy recipient/template/form model is exposed.

- [ ] **Step 3: Implement handler with safe JSON structures**

Emit campaign `id`, `name`, `status`, `created_at`, `launch_date`, and aggregate `result_count` only. Authenticate before route dispatch and return generic error bodies.

- [ ] **Step 4: Run focused tests and commit**

Run `go test ./controllers/control`; commit with `feat: add private engine control API`.

## Task 6: Describe isolated tenant-engine deployment

**Files:**
- Create: `deploy/tenant-engine/docker-compose.yml`
- Create: `deploy/tenant-engine/.env.example`
- Create: `deploy/tenant-engine/nginx.conf.template`
- Create: `deploy/tenant-engine/README.md`

**Interfaces:**
- Consumes: per-tenant engine isolation constraints in the Spec.
- Produces: an operator-editable, non-deploying reference topology.

- [ ] **Step 1: Add static topology checks in README**

List the invariants: one compose project per tenant, an isolated named network, a separate engine database, reference-only secret variables, and no shared mail credential variable.

- [ ] **Step 2: Add Docker Compose template**

Define `engine` and `database` services with `AWARENOW_TENANT_SLUG`, `AWARENOW_CONTROL_TOKEN`, `ENGINE_DATABASE_URL`, `DELIVERY_CREDENTIAL_REF`, and `CAMPAIGN_DOMAIN` sourced from environment variables. Name the network `awarenow-${AWARENOW_TENANT_SLUG}`.

- [ ] **Step 3: Add safe examples and proxy template**

Use placeholder-only values such as `replace-with-secret-reference`; mark the control path private in the Nginx template.

- [ ] **Step 4: Validate and commit**

Run `docker compose -f deploy/tenant-engine/docker-compose.yml config` only when Docker is installed; otherwise run a PowerShell YAML syntax/readability check and record the limitation. Run `git diff --check`; commit with `feat: add isolated tenant engine deployment template`.

## Task 7: Implement tenant scope and safe-event domain services

**Files:**
- Create: `control-plane/src/tenancy/principal.ts`
- Create: `control-plane/src/tenancy/tenantScope.ts`
- Create: `control-plane/src/tenancy/tenantScope.test.ts`
- Create: `control-plane/src/events/eventService.ts`
- Create: `control-plane/src/events/eventService.test.ts`

**Interfaces:**
- Consumes: Task 2 contracts and Task 4 `parseSafeEngineEvent`.
- Produces: `requireTenantScope(principal: TenantPrincipal): string` and `recordSafeEvent(input, principal, repository): Promise<SafeEngineEvent>`.

- [ ] **Step 1: Write failing tenant scope tests**

Cover tenant member scope resolution, platform administrator without selected tenant failure, and an attempt to use a caller-supplied tenant ID failure.

- [ ] **Step 2: Write failing event service tests**

Use an in-memory repository fake to prove the authenticated engine tenant reaches the repository, duplicate external event IDs are idempotent, and a rejected unsafe event never reaches storage.

- [ ] **Step 3: Implement minimal domain services**

Use `TenantPrincipal` with `subjectId`, `role`, `tenantId`, and `engineTenantId`; use the authenticated engine tenant ID, not payload tenant ID, for event storage.

- [ ] **Step 4: Run focused tests and commit**

Run the two test files using the eventual control-plane test command if available, otherwise record the command required by the integration task; commit with `feat: add tenant-scoped safe event service`.

## Task 8: Build tenant-aware dashboard components

**Files:**
- Create: `web/src/control-plane/types.ts`
- Create: `web/src/control-plane/TenantContextBanner.tsx`
- Create: `web/src/control-plane/AwarenessOverview.tsx`
- Create: `web/src/control-plane/ProvisioningStatus.tsx`
- Create: `web/src/control-plane/controlPlane.css`
- Create: `web/src/control-plane/TenantContextBanner.test.tsx`

**Interfaces:**
- Consumes: `TenantLifecycle` and safe aggregate metrics from Task 2.
- Produces: reusable components accepting a `tenant` object and no direct API calls.

- [ ] **Step 1: Write a component test**

Assert the banner renders the tenant display name, slug, and `PROVISIONING` status. Assert no engine token or secret is accepted in its prop type.

- [ ] **Step 2: Define presentation-only types**

Define `TenantView` with `id`, `displayName`, `slug`, and lifecycle `PROVISIONING | ACTIVE | SUSPENDED | FAILED`; define `AwarenessMetrics` with sent/opened/clicked/reported/trainingCompleted totals only.

- [ ] **Step 3: Implement visual building blocks**

Build a compact tenant context banner, an aggregate metric grid, and a lifecycle status panel with clear empty/loading-ready copy. Use existing UI primitives and AwareNow styles; do not add a second router, auth implementation, or API client.

- [ ] **Step 4: Run existing web checks and commit**

Run `npm run lint` and `npm run build` from `web/`; commit with `feat: add tenant-aware dashboard components`.

## Task 9: Integrate the first vertical slice

**Files:**
- Create: `control-plane/package.json`, `control-plane/tsconfig.json`, `control-plane/.env.example`
- Create: `control-plane/src/app.ts`, `control-plane/src/server.ts`, `control-plane/src/config/env.ts`
- Create: `control-plane/src/engine/engineClient.ts`, `control-plane/src/provisioning/provisionTenant.ts`, `control-plane/src/analytics/overview.ts`
- Create: `control-plane/src/app.test.ts`, `control-plane/README.md`
- Modify: `web/src/App.tsx`, `web/src/App.css`, `README.md`, `.github/workflows/ci.yml`
- Modify only if needed for registration: `gophish.go` or the established Go router registration file

**Interfaces:**
- Consumes: every preceding task's public interfaces exactly as written.
- Produces: a locally buildable Express service and an integrated, static current-tenant dashboard path.

- [ ] **Step 1: Write failing integration tests**

Use a dependency-injected app factory and test `GET /api/v1/health`, `GET /api/v1/tenants/current` with a tenant principal fixture, `POST /api/v1/events/engine` rejecting `password`, and `GET /api/v1/analytics/overview` returning only the fixture tenant's totals.

- [ ] **Step 2: Create control-plane tooling**

Use Node 24, TypeScript, Express, Zod, Prisma, Vitest, Supertest, and `tsx`. Define scripts `typecheck`, `test`, `test:run`, `build`, and `dev`; `build` must compile without a running database.

- [ ] **Step 3: Wire configuration and routes**

Validate `PORT`, `DATABASE_URL`, `AWARENOW_CONTROL_TOKEN`, and `CONTROL_PLANE_BASE_URL`; do not print values. Inject repositories and principals for tests. Register health, tenant current, event intake, and aggregate analytics routes.

- [ ] **Step 4: Add engine adapter and provisioning workflow**

Implement an adapter that sets the private bearer token, parses only `EngineHealth` and `SafeCampaignSummary`, and redacts errors. Implement idempotent state transitions `PROVISIONING -> ACTIVE` or `FAILED` with audit entries; do not invoke Docker, create databases, or send mail in this increment.

- [ ] **Step 5: Integrate UI and Go route registration**

Mount the tenant banner, overview, and lifecycle panel in the existing dashboard with static safe fixture data behind an explicit development-only adapter. Register Go control handlers only through the established router and only when `AWARENOW_CONTROL_TOKEN` is configured.

- [ ] **Step 6: Verify, reconcile CI, and commit**

Run `npm run lint` and `npm run build` in `web/`; run `npm run typecheck`, `npm run test:run`, and `npm run build` in `control-plane/`; run `go test ./controllers/control`; run `git diff --check`. Update CI to add the control-plane checks. Commit with `feat: integrate multi-tenant control plane foundation`.

## Review and completion

- [ ] Dispatch one scoped reviewer for each swarm task, addressing Critical/Important findings before integration.
- [ ] Dispatch the dedicated integration engineer only after all eight worker reports are available.
- [ ] Run a final whole-branch review of the feature worktree; make one reviewed fix wave if necessary.
- [ ] Do not merge, push, deploy, or delete this worktree without explicit user direction.
