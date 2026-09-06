# Phishing Assessment Enhancement — Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close AwareNow's two biggest competitive gaps as an email-only phishing **assessment** tool — a real reporting loop (native report button + real/unknown-phish intake) and complete analytics export (PDF/XLSX) — without adding SMS or voice/vishing capability, per explicit scope decision on 2026-09-05.

**Architecture:** Two independent tracks that can run in parallel, plus one small sequential chain:
- **Track A (sequential, Tasks 1-4):** a new `ReportedMessage` intake path for *real, non-campaign* suspicious emails, landing on the existing public `PhishingServer` (`controllers/phish.go`), reviewed and converted into a draft `Template` through the admin API and a new web UI page.
- **Track B (independent, Task 5):** an Outlook add-in that calls AwareNow's **existing, already-wired** `/report` endpoint (`ReportHandler` in `controllers/phish.go`, calls `models.Result.HandleEmailReport`) — this endpoint already works and is CORS-open specifically for external reporting clients; nothing on the Go side is required for the known-campaign-report path, only a client.
- **Track C (independent, Tasks 6-7):** PDF and XLSX analytics export, replacing the current `501 Not Implemented` stubs in `controllers/api/analytics.go`.

**Tech Stack:** Go (existing stack: gorilla/mux, gorm/sqlite), vanilla JS + Office.js for the Outlook add-in (no new frontend build tooling), `github.com/xuri/excelize/v2` (XLSX, pure Go) and `github.com/go-pdf/fpdf` (PDF, pure Go) as new Go dependencies.

**Spec:** This plan implements Phase 1 (items A, B, C) of the enhancement roadmap discussed in-session on 2026-09-05, itself derived from a feature-by-feature comparison of AwareNow against KnowBe4, Proofpoint, Cofense, Hoxhunt, and Microsoft Defender ASR (chat record; no separate spec file exists). Scope explicitly excludes SMS/smishing delivery and voice/vishing per the user's instruction.

## Global Constraints

- No new cgo dependencies. This repo already has one cgo pain point (`bitbucket.org/liamstask/goose` → `sqlite3.Error`, see `docs/development.md`) that breaks local builds on machines without a C compiler (this dev machine included). Both new libraries (`excelize`, `go-pdf/fpdf`) are pure Go — verify this holds before adding anything else.
- Follow existing patterns: `models.Response{Success, Message}` + `api.JSONResponse(w, v, status)` for all JSON responses; `db.Save`/`gorm.ErrRecordNotFound` for persistence (see `models/enhanced_tracking.go` for the idiom); `mid.Use(handler, middlewares...)` for chained middleware (see `controllers/api/server.go`).
- The public, unauthenticated intake endpoint (Task 2) must be rate-limited — reuse `middleware/ratelimit.PostLimiter`, the same type already used to rate-limit `/login` in `controllers/route.go`.
- `go test ./...` is known-broken on this Windows dev machine for any package importing `models` (cgo sqlite3 issue, pre-existing, unrelated to this work — see `docs/development.md`). Verify new Go tests via `go vet` and code review locally; treat GitHub Actions (`ubuntu-latest`, has gcc) as the real test gate for anything under `models`/`controllers`.
- Keep this file updated as work ships: check off steps as completed, and append a dated line to the Progress Log below for every task that ships (merged/pushed), including what changed and any deviation from the plan as written.

---

## Progress Log

_Append one line per shipped task. Do not edit history above this point — add new lines below._

- 2026-09-05 — Plan created. No tasks shipped yet.
- 2026-09-05 — Task 1 (ReportedMessage model) shipped. Deviated from the plan's illustrative test code: this codebase uses gocheck (ModelsSuite/check.C), not testing.T + setupTest/tearDown as the plan assumed — tests were written against the real convention instead.
- 2026-09-05 — Task 2 (public intake endpoint + ParseEmailContent helper) shipped. Deviated from the plan's illustrative test code again: controllers/ tests use plain testing.T with a real httptest.Server (setupTest/tearDown in controllers_test.go) and real HTTP calls, not gocheck and not direct ServeHTTP calls. While verifying this, also fixed unrelated pre-existing breakage that was blocking it: controllers/api had several compile errors (undefined models.NowUTC, a uint/int64 JWT UserID mismatch, DeliveryConfig/CampaignSMTP field-name mismatches, a csv.Writer.Flush() misuse) and the smtp/campaigns/results tables were missing columns their Go structs already expected (the real migrations for those existed only under db/migrations/, a path this app never actually reads — relocated to db/db_sqlite3/migrations/ and db/db_mysql/migrations/). Also fixed two stale test assertions in controllers/phish_test.go's clickLink helper that predated the tracking-script-injection feature.
- 2026-09-05 — Task 3 (admin review API) shipped. Same test-convention correction as Tasks 1-2 (real ServeHTTP + Bearer-token requests, not the plan's pseudocode). Also found the built-in "user" role already has PermissionModifyObjects — the permission-denial test needed a user with zero role_permissions rows, not the existing createUnpriviledgedUser helper, to actually hit the 403 path.
- 2026-09-05 — Task 4 (Reported Messages web UI) shipped. Built this repo's first test harness for a useQuery-based component (a per-test QueryClientProvider wrapper) since none existed. Confirmed the new API client correctly reads raw response bodies rather than copying a pre-existing, out-of-scope bug in web/src/api/templates.ts that assumes a {success,message,data} envelope the real backend doesn't send for that endpoint.

---

## Enhancement Roadmap Update — 2026-09-06

**Status:** P0 is implemented and verified locally as recorded below; later milestones remain planned and no additional release is claimed.
This section sets the next execution order and supplements Tasks 5–7 below.
Keep the completed Tasks 1–4 and their historical progress entries intact.
Retain the existing email-assessment scope; SMS and voice remain excluded.

**Current product boundary (user clarification, 2026-09-06):** Both platforms
operate independently. AwareNow owns phishing assessments, reporting, analytics,
and its own AI configuration; AwareCheck retains its existing assessment and
training capabilities. AwareNow does not build training modules. No shared login,
employee synchronization, training requests, completion callbacks, or runtime
dependency on AwareCheck is in the current delivery scope. The integration design
below is preserved only as a deferred future option requiring a new go-ahead.

### Pre-P0 baseline and evidence

- Tasks 1–4 are recorded as shipped; the report model, intake handler, review
  API, and React review page are present in the repository.
- `models/reported_message.go` has no owner/tenant field; its list and lookup
  queries are global. The review routes require `PermissionModifyObjects`,
  which is insufficient to establish ownership of an individual report.
- In `controllers/api/reported_message.go`, approval creates the template
  before updating the report and returns success even if that update fails.
  Rejection does not enforce a pending-only transition.
- `controllers/phish.go` accepts unknown reports with a caller-supplied email;
  the handler has no explicit request-body size cap or duplicate detection.
- `controllers/api/analytics.go` still returns 501 for PDF/XLSX. The existing
  Progress Log also records an unresolved template-client response mismatch.
- `control-plane/src/serverDependencies.ts` explicitly supplies development-only
  authentication and repositories. The management plane is a foundation,
  not a completed production integration.

These record the source-review findings before the P0 changes below. Historical
Windows/cgo notes above are not evidence of this checkout's current build
status; reproduce failures before choosing a dependency fix.

### P0 — Reliable reporting and a verified baseline

**Execution status: COMPLETE locally (2026-09-06).** Implementation and verification finished in the working tree.
Completion means implemented and verified in the working tree; release/shipping
is tracked separately. Unchecked later phases remain planned, not ongoing.

| Workstream | Status | Verification / scope |
| --- | --- | --- |
| Baseline Go and control-plane checks | COMPLETE locally | Go tests/build; control-plane 44 tests/typecheck/build passed |
| Report ownership and atomic review | COMPLETE locally | Owner-negative, concurrent approval, rollback, and legacy quarantine regressions passed |
| Intake limits, idempotency, CORS | COMPLETE locally | Explicit configured owner; size, validation, retry, preflight, and rate-limit regressions passed |
| Frontend template API contract | COMPLETE locally | Web lint, 9 tests, and build passed |
| P1, standalone USP, AI configuration, P2/P3 | PLANNED | Begin after applicable prerequisites |
| AwareCheck integration I1–I3 | DEFERRED | Excluded from current implementation |

**Outcome:** Reviewers can trust report ownership and review results before
report intake is rolled out more broadly. Suggested ownership: Go/API engineer
with frontend support. Complete this milestone before the Outlook pilot.

- [x] Run root `go test ./...` and `go build`; run `npm ci`, lint, tests, and
  build in `web/`; run install, tests, typecheck, and build in `control-plane/`.
  Record actual failures and fix reproducible blockers. Reconcile stale plan
  statuses with evidence rather than checking them off from file presence.
- [x] Define report ownership: use explicit engine/owner binding now and align
  it with the [tenant contracts](../../architecture/control-plane-contracts.md).
  Derive scope from trusted server configuration or authenticated identity,
  never a submitted tenant ID or reporter email. Specify how existing unowned
  reports are migrated or restricted to a designated administrator.
- [x] Scope list, detail, approve, and reject queries consistently. Add tests
  showing a second owner cannot read or mutate the first owner's reports.
- [x] Make template creation and pending-to-approved transition transactional.
  Use a conditional status update for approve/reject; handle conflicts, missing
  records, and database failures explicitly.
- [x] Add body/field limits, input validation, and scoped retry deduplication
  without merging different reporters. Treat submitted identity as unverified
  until authenticated. Preserve rate limiting and test actual CORS preflights.
- [x] Fix the documented template response mismatch and add an API-contract
  regression test for the report-to-template workflow.

**Acceptance:** Concurrent approvals create exactly one template; rollback
leaves no orphan template; approval/rejection cannot overwrite a completed
decision; oversized requests are rejected; ownership-negative tests pass.

**Local verification — 2026-09-06:** `go test ./...` and `go build`
passed; web `npm ci`, lint, all 9 tests, and build passed; control-plane
`npm ci`, all 44 tests, typecheck, and build passed. Report API tests also
passed with `-race`. The reproduced Go failure was an Office ZIP fixture
comparing compressed bytes; its test now compares every entry's exact
uncompressed content. No Goose/SQLite compile failure occurred here.
The existing web build warns about unprocessed Tailwind directives; that
configuration issue remains outside this reporting milestone. Other campaign
resource clients retain their pre-existing response-envelope assumptions.
Control-plane installation reports three high-severity dependency findings.
See [intake configuration and legacy-report recovery](../../report-intake.md).
No commit, push, release, or mailbox pilot is claimed by this local completion.

### P1 — Finish the reporting loop and usable exports

**Outcome:** Employees can report mail and administrators can review and share
assessment results. Dependencies: P0 for reporting; export implementation may
proceed independently against verified owner-scoped analytics.

- [ ] Refresh Task 5's Outlook design before implementation. Microsoft provides
  an integrated spam-reporting extension with client-specific support:
  [official implementation and supported-client guidance](https://learn.microsoft.com/en-us/office/dev/add-ins/outlook/spam-reporting).
  Record the chosen manifest, supported clients, permissions, and fallback;
  do not assume the older illustrative manifest covers every client.
- [ ] Complete Task 5 with known-campaign and unknown-message paths, trusted
  reporting-endpoint validation, clear success/failure feedback, and safe
  retries. Test rid extraction against unrelated links, not just positive cases.
  Run a controlled mailbox pilot before marking client integration complete.
- [ ] Improve `web/src/pages/ReportedMessages/ReportedMessageList.tsx` with
  server pagination, status/date/search filters, detail inspection, and visible
  query/mutation errors. Preview untrusted content as text by default; any HTML
  preview must prevent scripts, external loads, and active navigation.
- [ ] Complete Tasks 6–7 and expose PDF/XLSX downloads in the UI. Verify library
  compatibility with the supported Go matrix before selecting versions. Use
  the same filters, time zone, and metric definitions across screen and exports.
  Prevent spreadsheet formula interpretation of untrusted fields.

**Acceptance:** Report → review → draft template works end to end without
launching a campaign. Tests cover report retry, preview safety, and export
authorization. Open generated PDF/XLSX files and verify empty, large, Unicode,
and formula-like inputs; exported totals match the filtered dashboard fixture.

### P2 — Better assessment insight and administration

**Outcome:** Administrators can identify improvements across campaigns and
operate reporting at a predictable cost. Start after a successful P1 pilot.

- [ ] Add campaign/cohort comparisons, report rate, median time to report, and
  repeat-click trends. Define denominators, attribution windows, and handling
  of missing data; flag suspected automated interactions without silently
  deleting raw observations. Avoid treating email opens as confirmed behavior.
- [ ] Add reviewer assignment, notes, reason codes, and an audit trail. Keep
  threat classification separate from approval to reuse a message as a template.
- [ ] Add configurable retention for raw reported content, redaction before
  template reuse, and authorized deletion with a documented backup policy.
- [ ] Add request correlation, intake/review failure metrics, queue/backlog
  monitoring, and a tested backup/restore runbook. Keep message bodies and
  secrets out of operational logs.

**Acceptance:** A deterministic campaign fixture produces documented metrics;
audit records identify actor/action/time; retention tests cover expiry and
owner isolation; a restore exercise recovers a usable test instance.

### P3 — Production multi-tenant management

Continue the existing [control-plane implementation plan](2026-09-05-awarenow-multitenant-control-plane.md)
instead of creating a competing implementation track. This milestone can move
earlier if serving multiple organizations becomes the immediate requirement.

- [ ] Replace development dependencies with verified authentication, persistent
  repositories, durable audit storage, and restart-safe provisioning jobs.
- [ ] Establish explicit engine control-owner identity before mounting private
  Go control routes; connect real tenant-aware UI data only after that boundary.
- [ ] Add credential rotation, retry/idempotency handling, readiness checks,
  tenant suspension, and migration/recovery procedures.

**Acceptance:** Two-tenant integration tests deny cross-tenant access; data and
audit history survive restart; provisioning retries do not duplicate engines;
suspended tenants cannot perform prohibited operations. Demo fixtures and
development authentication are excluded from production configuration.

### Product USP — AwareNow Proof of Resilience

**Proposed positioning:** “Turn the threats your people report into measurable
proof that your organization is getting better at recognizing them.”

**Target customer hypothesis:** Security teams and assessment consultancies
that need defensible before/after evidence, with customer-controlled hosting.
Validate this with five buyer interviews before investing in a broad suite.

The flagship feature is an **Assessment Lab**: turn a reviewed incident into a
sanitized scenario, define the recognition skill being tested, run a controlled
assessment, and export an evidence-backed result. This extends the existing
reported-message → template workflow into a measurement product.

#### Competitive rationale and limits

AI-generated messages, adaptive simulations, and report-to-template conversion
are not sufficient differentiation. First-party sources reviewed on 2026-09-06:

- [Hoxhunt phishing training](https://hoxhunt.com/product/phishing-training)
  describes adaptive training using AI and behavioral data.
- [KnowBe4 security awareness training](https://www.knowbe4.com/products/security-awareness-training)
  describes personalized simulations and automated coaching.
- [KnowBe4 SOC 3 report](https://www.knowbe4.com/hubfs/KnowBe4-2026-Type-2-SOC-3-Final-Report.pdf)
  describes PhishFlip conversion of user-reported attacks into simulations.

The differentiation hypothesis is the combined workflow: customer-specific
incident provenance, explicit experimental design, legitimate-message controls,
transparent uncertainty, and a reproducible evidence export in a self-hostable
product. This source scan does not establish market exclusivity. Validate the
combination in competitor demos and buyer interviews before claiming uniqueness.

#### Flagship workflow

1. A reviewer selects a reported incident, removes sensitive content, and
   replaces active URLs/resources with controlled simulation assets.
2. The reviewer tags a specific skill, such as identifying a sender-domain
   mismatch, and creates versioned scenarios with comparable difficulty.
3. An administrator defines cohorts, baseline, observation window, success
   metric, and comparable baseline/follow-up assessment conditions.
4. Participants receive approved simulated threats and, optionally, clearly
   governed benign control messages. Responses are measured using the same
   observation window; nonresponse is not automatically counted as safe behavior.
5. A follow-up assessment uses an unseen variant to test transfer of the skill,
   rather than recognition of the original template.
6. The result explains observed change, uncertainty, and limitations, and exports
   the underlying definitions and aggregate counts for independent review.

Example demonstration: a reported invoice impersonation becomes a sanitized
scenario. Comparable cohorts receive baseline and follow-up assessments using
different scenario variants. The evidence report compares reporting before risky
interaction and unnecessary reports of benign controls. The standalone product
reports observed changes without claiming that a training intervention caused them.
No result percentages should be shown as real until measured in a pilot.

#### What the evidence report must show

- **Recognition:** Share of eligible simulated-threat recipients who report
  before a defined risky interaction within the observation window.
- **Discrimination:** Benign-control reporting rate, displayed separately from
  threat reporting. Do not use it to discourage reporting uncertain real mail.
- **Recovery:** Reports made after a risky interaction, counted separately from
  early detection so improvement is visible without rewriting the event history.
- **Speed:** Time-to-report distribution plus the nonreporting proportion;
  distinguish SMTP acceptance from confirmed delivery when delivery is unknown.
- **Evidence quality:** Cohort sizes, exclusions, scenario versions, event
  classification rules, missing observations, and uncertainty intervals.

Avoid an opaque individual “risk score.” Report group outcomes; suppress small
cohorts using a documented threshold. Suspected scanner activity remains labeled
and auditable, with raw-versus-filtered aggregate results. A before/after change
alone is observational; causal wording requires a justified comparison design.

#### MVP implementation sequence

Keep P0 as the first dependency. Build this flagship after P1's reporting and
export foundation, ahead of the broader P2 feature list; reuse P2 metrics work.

- [ ] **USP-1: Measurement specification.** Define metric denominators, event
  precedence, deduplication, eligibility, missing-data handling, and uncertainty
  methods. Validate calculations on hand-checkable fixtures. Review experiment
  design for baseline differences and contamination between cohorts.
- [ ] **USP-2: Provenance and scenarios.** Add owner-scoped assessment/scenario
  records referencing reported-message and template IDs, sanitized versions,
  skill tags, and reviewer approval. Retain only provenance necessary for audit.
  Verify sanitized scenarios contain no live malicious destinations or secrets.
- [ ] **USP-3: Assessment orchestration.** Persist cohort assignments, campaign
  links, assessment conditions, and observation windows. Start with explicit
  administrator setup and one recognition skill; no autonomous campaign launch.
  Verify ownership, retry safety, and repeatable assignment.
- [ ] **USP-4: Evidence view and export.** Extend Go analytics and the React UI
  with baseline/follow-up comparisons, benign controls, uncertainty, and explicit
  limitations. Export a versioned JSON evidence bundle plus PDF/XLSX summary.
  Independent recomputation from exported aggregate counts must match the UI.
- [ ] **USP-5: Buyer pilot.** Use an authorized test environment first, then
  evaluate with three consenting design partners. Require at least two partners
  to identify a concrete decision the evidence improves and express willingness
  to pay or renew. Record setup time and assessment-to-report effort against
  their existing process. These are proposed validation gates, not achieved results.

**MVP boundary:** One email-based recognition skill, one baseline/follow-up
comparison, optional benign controls, manual scenario review, and one evidence
report. AI can later assist with sanitized scenario drafts or explanations;
it is not the source of truth for metrics, attribution, or campaign approval.

**Release gate:** P0/P1 checks pass; repeated imports do not inflate counts;
two-owner tests enforce isolation; scanner/missing-event fixtures produce honest
results; exports reproduce dashboard values; insufficient evidence is displayed
instead of an unsupported improvement claim. Publish a synthetic demonstration
with reproducible inputs before making marketing performance claims.

### Platform-admin AI configuration — DeepSeek and CommandCode

**User requirement:** The platform administrator adds API credentials and selects
the provider and model through dropdowns. Support DeepSeek and CommandCode first.
Prefer CommandCode's available free models for initial use. This is planned
configuration work, not an API subscription, credential setup, or live API call.

#### Admin experience

- [ ] Add **Platform Settings → AI Providers** with provider dropdown
  (`CommandCode`, `DeepSeek`), masked API-key input, enabled state, connection
  status, **Test Connection**, **Refresh Models**, and **Save** actions.
- [ ] Populate a searchable **Model** dropdown for the selected provider. Show
  display name, exact model ID, supported capability, availability, and pricing
  status (`Free`, `Paid`, `Unknown`) with its last verification time. Filter out
  models unsupported by the implemented text-generation adapter.
- [ ] Allow an administrator to set the platform default provider/model and an
  allowed-model list. Persist provider and model together; switching provider
  clears an incompatible model selection. Tenant users cannot modify platform
  credentials or bypass the allowed-model policy.
- [ ] Default initial setup to CommandCode with **Free models only** enabled.
  Require an explicit model choice from the verified free options; do not guess
  a permanent default model ID. If none are available, leave AI unavailable with
  an actionable message while manual assessment features continue to work.
- [ ] Support replace/revoke credentials without returning the stored key.
  Display usage totals, rate-limit errors, and selected model status. Paid use
  requires an explicit administrator policy change; never automatically fall
  back from a free model to a paid model or to DeepSeek.

#### Provider contracts and free-model verification

Official references checked on 2026-09-06:

- [DeepSeek API](https://api-docs.deepseek.com/) and
  [model discovery](https://api-docs.deepseek.com/api/list-models): use the
  documented authenticated model listing and text-generation API. Keep model
  IDs dynamic rather than embedding today's catalog in the UI.
- [CommandCode Provider API](https://commandcode.ai/docs/provider): base URL
  `https://api.commandcode.ai/provider/v1`, discovery via `GET /models`.
  Implement Chat Completions-compatible text models first; exclude models
  requiring a different endpoint until their adapter is implemented.
- [CommandCode pricing](https://commandcode.ai/docs/resources/pricing-limits):
  verify current model pricing and account entitlement separately. A zero-cost
  model does not establish that the account's API access is free. Promotions,
  capacity limits, and plan eligibility can change.

Do not infer price from a model-name suffix or assume the models endpoint
includes pricing. Use authoritative pricing metadata when available; otherwise
maintain a reviewed, expiring catalog with source and verification timestamp.
Free-only mode blocks paid, unknown-price, or stale entries. Recheck eligibility
before generation and surface unavailable/promotional-expiry errors. Application
checks reduce unexpected charges but cannot guarantee provider billing; enforce
provider-side spending limits where supported.

#### Backend and implementation checklist

- [ ] **AI-1: Authorized configuration.** Implement platform-admin-only settings
  endpoints and durable configuration in the TypeScript control plane, using
  verified authentication rather than its development principal stubs. This
  requires the relevant authentication/persistence slice of P3, not all tenant
  provisioning. Add server-side role checks to every configuration mutation.
- [ ] **AI-2: Secrets and adapters.** Store keys through a secret manager or
  encrypted storage with the encryption key outside the database. Make all
  provider calls server-side through explicit DeepSeek/CommandCode adapters;
  use fixed official hosts initially. Never expose keys in browser storage,
  responses, logs, analytics, or exported reports.
- [ ] **AI-3: Catalog and policy.** Implement cached model discovery, explicit
  refresh, capability filtering, pricing verification, and server-enforced
  provider/model allowlists. Handle revoked keys, removed models, and unavailable
  catalogs without silently changing the selected model.
- [ ] **AI-4: Assessment assistance.** Use the chosen model for reviewed scenario
  drafts, skill-tag suggestions, and explanations of computed assessment
  results. Send only sanitized, necessary inputs. Treat reported-email content
  as untrusted data; AI output cannot execute tools, approve or launch campaigns,
  calculate authoritative scores, or create AwareCheck training content.
- [ ] **AI-5: Limits and verification.** Add bounded timeouts, rate limits,
  concurrency/output-token caps, and limited retries. Audit configuration
  changes and request provider/model/status/token usage without prompt bodies.
  Test administrator versus tenant permissions, key masking/rotation, model
  switching, discovery failure, pricing expiry, and rejection of paid fallback.
  Use mocked providers for automated tests; a manual connection test must
  distinguish credential validation from an optional generation test.

**Acceptance:** An administrator can configure either provider, refresh and
select a compatible model, save it, and generate a reviewed assessment draft
through that exact provider/model. Settings survive restart, tenant users cannot
retrieve credentials, and free-only mode rejects paid/unknown/stale models.
Document any provider account requirement before the first live use.

### Deferred future plan — AwareCheck integration (not current scope)

**Status: Deferred by user instruction, 2026-09-06.** Preserve this research for
future planning only. None of the integration checklists or milestones below
is authorized for current implementation or a prerequisite for P0–P3, AI settings,
or the standalone USP MVP. Each platform keeps its own users, authentication,
data, configuration, reporting entry point, and release lifecycle. In particular,
AwareNow's own Outlook reporting work remains in P1. Revalidate both repositories
and agree on a new integration phase before activating this design.

**Review scope:** Read-only inspection of local sibling checkout
`../AwareCheck`, remote `https://github.com/fir3storm/awarecheck.git`, HEAD
`10d1f49`. The checkout contains extensive uncommitted changes; findings describe
that working tree, not verified remote or deployed behavior. No AwareCheck files
were changed and no live integration or test suite was run for this review.

#### Existing AwareCheck capabilities to reuse

Paths below are relative to the AwareCheck repository:

| Capability found | Source | Integration consequence |
| --- | --- | --- |
| Tenant-scoped employees with string IDs; email unique per tenant | `apps/api/src/database/models.py`, `User` | Keep AwareCheck as employee identity authority |
| Employee category scores, risk/tier, workforce timeline, training history | `apps/web/src/features/workforce/WorkforceEmployeeAnalytics.tsx`, `types.ts`; `apps/api/src/modules/reports/service.py` | Enrich existing employee detail pages |
| Suggestions, AI priority plans, assignment deduplication, lessons | `apps/api/src/modules/training/service.py`, `priority_plan_service.py`, `suggestion_router.py` | Reuse the existing recommendation and assignment loop |
| Recommendation plans require internal attempt/campaign foreign keys | `apps/api/src/database/models.py`, `TrainingRecommendationPlan` | Add an external-source path; do not manufacture quiz attempts |
| HMAC-signed outbound webhooks | `apps/api/src/modules/integrations/service.py`, `schemas.py` | Extend delivery contracts; current supported events omit `training.completed` |
| Outlook reporting and confirmed-report category boosts | `apps/outlook-addin/`; `apps/api/src/modules/email_reports/scoring.py` | Coordinate intake and prevent duplicate reporting credit |

AwareCheck also owns knowledge tests and their reassessments. AwareNow adds
observed email-simulation behavior and independent phishing reassessment; it
does not replace AwareCheck's existing assessment or training features.

#### Employee experience and event loop

1. Bind the two tenants and map each AwareNow recipient to an existing AwareCheck
   `user_id`. Use tenant-scoped email only for initial reviewed matching; retain
   stable IDs after mapping. Quarantine ambiguous/unmatched recipients. Handle
   email changes, inactive users, deletion, and tenant disconnection explicitly.
2. AwareNow records an outcome and emits a minimal authenticated event. AwareCheck
   adds it to that employee's existing timeline and a new **Phishing behavior**
   section: campaign/scenario, observed action, event confidence, reporting
   latency, trend, and related training. Retain provenance and correction history.
3. Map reliable behavior plus scenario skill tags to AwareCheck categories.
   Current indices include Phishing `0`, Spear Phishing `1`, BEC `5`, Credential
   Theft `6`, Social Engineering `7`, and Phishing Awareness `22`. Publish a
   versioned semantic mapping; do not infer category from the click alone.
4. AwareNow requests a recommendation referencing the evidence; AwareCheck
   creates or updates its Suggested Interventions entry and ranks appropriate
   lessons. Suggestions should appear promptly after accepted reliable evidence,
   with a provisional target of 60 seconds at agreed pilot load. AI may improve
   rationale/ranking; deterministic matching remains available when AI is down.
5. Existing HR/admin approval assigns training in AwareCheck. Optional automatic
   assignment is a separate tenant opt-in with approved categories, existing
   lesson allowlists, cooldowns, and active-assignment deduplication. AwareNow
   displays the recommendation and assignment status with an AwareCheck deep link.
6. AwareCheck emits assignment/completion updates; AwareNow offers an approved
   reassessment using a new scenario. Results flow back to the same employee
   profile. Training completion alone is not evidence of improved behavior.

Example: a reliable form-submission event in an approved credential-theft
scenario creates a Credential Theft recommendation in AwareCheck. Existing
matching training is reused instead of duplicated. Completion becomes visible
in AwareNow, and a later phishing reassessment enriches the employee's timeline.
Transmit the submission outcome only, never submitted passwords or field values.

#### Proposed contract and consistency rules

- [ ] Define versioned AwareNow events such as `assessment.outcome.recorded` and
  `assessment.outcome.corrected`. Envelope: schema version, globally unique
  event ID, tenant binding, mapped employee reference, campaign/scenario IDs,
  recipient-attempt reference, occurrence time, outcome, confidence/classification,
  skill tags, and optional superseded-event ID. Validate a strict safe-field list.
- [ ] Add a tenant-bound service credential and proposed AwareCheck ingestion
  endpoint under `/api/v1/integrations/awarenow/`; existing integration routes
  manage outbound webhooks and are not an external outcome-ingestion API.
  Bind tenant identity to credentials, reject replay/stale signatures, and use
  durable outbox/inbox storage with unique source-event constraints and retries.
- [ ] Add `training.completed` to AwareCheck's outbound event registry and emit
  it transactionally from actual completion, alongside existing `training.assigned`.
  Include external evidence/reference IDs and assignment IDs for correlation.
  Add authenticated reconciliation for missed events; handle out-of-order updates.
- [ ] Extend recommendation provenance to support internal quiz attempts OR
  external assessment evidence, with database constraints and corresponding
  service/UI changes. Preserve existing quiz behavior and dedupe across sources.
- [ ] Preserve AwareCheck's existing quiz scores/tier initially. Show behavioral
  evidence alongside them, not as an invented exam percentage. Any later combined
  score requires an explicit versioned scoring policy and validation. Coordinate
  existing confirmed-report boosts to credit a shared report only once.
- [ ] Reuse AwareCheck's Outlook reporting entry point where practical: route
  validated AwareNow campaign reports to AwareNow; keep real-email review in
  AwareCheck with explicit forwarding for sanitized scenario reuse. Use shared
  correlation IDs; choose a single intake owner before shipping two report buttons.
- [ ] Keep AI credentials within each product's backend. AwareNow sends evidence
  and recommendation requests, not its platform API keys; AwareCheck retains
  control of its lesson catalog, language, ranking, and training generation.

#### Integration milestones and acceptance

**I1 — Readable employee evidence:** Tenant mapping, durable event ingestion,
profile section, and timeline. Test two-tenant isolation, renamed employees,
unmapped IDs, replay, corrections, and deletion. Repeated delivery creates one
logical observation; uncertain scanner events never trigger training.

**I2 — On-the-fly suggestions:** External recommendation provenance, category
mapping, AwareCheck ranking/fallback, and AwareNow status/deep links. Verify the
same employee/category does not receive duplicate active training and manual
approval remains the default. Measure the event-to-suggestion latency target.

**I3 — Completion and reassessment:** Completion webhook, reconciliation,
reassessment linkage, and shared before/after evidence. Demonstrate one mapped
employee from simulation outcome through AwareCheck completion to AwareNow
reassessment, including an outage/retry exercise without duplicated credit.

If integration is explicitly reactivated in the future, sequence I1 before I2
and I3 after verifying authenticated persistence in both products. These phases
extend the standalone USP; they do not block its release. When implementation touches AwareCheck, update
its required `docs/FEATURE_CATALOG.md` and focused workforce, suggestion,
completion, scoring, and webhook tests in the same change.

### Delivery and completion rules

Implement P0 first, then P1, then the USP MVP, then remaining P2 work; use the
separate control-plane plan for P3.
Schedule AI configuration after P0 and the platform-admin authentication and
persistence slice; it may proceed alongside P1. The USP's deterministic metrics
must remain usable without AI or AwareCheck. Exclude I1–I3 and all cross-platform
integration work from current delivery; training integration remains a future
option only.
Size each checklist item after reproducing the baseline; calendar estimates
remain unset until capacity and integration constraints are known. For every
completed item, record the commit, tests, and remaining limitations. Add a
shipped-task Progress Log entry only when it actually ships. This planning
update does not authorize deployment or sending a live campaign.

## Task 1: `ReportedMessage` data model

**Files:**
- Create: `models/reported_message.go`
- Test: `models/reported_message_test.go`

**Interfaces:**
- Produces: `models.ReportedMessage` struct; `models.CreateReportedMessage(rm *ReportedMessage) error`; `models.GetReportedMessages(status string) ([]ReportedMessage, error)`; `models.GetReportedMessageByID(id int64) (ReportedMessage, error)`; `models.ErrReportedMessageNotFound`; status constants `models.ReportedMessageStatusPending`, `models.ReportedMessageStatusApproved`, `models.ReportedMessageStatusRejected`.
- Consumes: nothing (foundational task).

- [x] **Step 1: Write the failing test**

```go
// models/reported_message_test.go
package models

import "testing"

func TestCreateAndGetReportedMessage(t *testing.T) {
	setupTest(t)
	defer tearDown(t)

	rm := &ReportedMessage{
		ReporterEmail: "alice@example.com",
		Subject:       "Your invoice is overdue",
		BodyText:      "Please click here to pay",
		BodyHTML:      "<p>Please <a href=\"http://evil.example\">click here</a> to pay</p>",
	}
	if err := CreateReportedMessage(rm); err != nil {
		t.Fatalf("CreateReportedMessage failed: %v", err)
	}
	if rm.ID == 0 {
		t.Fatal("expected ID to be set after create")
	}
	if rm.Status != ReportedMessageStatusPending {
		t.Fatalf("expected default status %q, got %q", ReportedMessageStatusPending, rm.Status)
	}

	got, err := GetReportedMessageByID(rm.ID)
	if err != nil {
		t.Fatalf("GetReportedMessageByID failed: %v", err)
	}
	if got.Subject != rm.Subject {
		t.Fatalf("expected subject %q, got %q", rm.Subject, got.Subject)
	}

	pending, err := GetReportedMessages(ReportedMessageStatusPending)
	if err != nil {
		t.Fatalf("GetReportedMessages failed: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending message, got %d", len(pending))
	}
}

func TestGetReportedMessageByIDNotFound(t *testing.T) {
	setupTest(t)
	defer tearDown(t)

	_, err := GetReportedMessageByID(999)
	if err != ErrReportedMessageNotFound {
		t.Fatalf("expected ErrReportedMessageNotFound, got %v", err)
	}
}
```

(`setupTest`/`tearDown` are the existing per-test sqlite fixtures already used throughout `models/*_test.go` — check `models/models_test.go` for the exact names in use if these differ.)

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./models/ -run TestCreateAndGetReportedMessage -v`
Expected: FAIL — `ReportedMessage`, `CreateReportedMessage`, etc. undefined.

- [x] **Step 3: Write the implementation**

```go
// models/reported_message.go
package models

import (
	"errors"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/jinzhu/gorm"
)

// ReportedMessageStatusPending indicates a reported message awaiting admin review.
const ReportedMessageStatusPending = "pending"

// ReportedMessageStatusApproved indicates a reported message that was converted into a template.
const ReportedMessageStatusApproved = "approved"

// ReportedMessageStatusRejected indicates a reported message an admin dismissed.
const ReportedMessageStatusRejected = "rejected"

// ErrReportedMessageNotFound indicates no reported message was found for the given criteria.
var ErrReportedMessageNotFound = errors.New("reported message not found")

// ReportedMessage stores a real (non-campaign) suspicious email a recipient
// reported through the Outlook add-in or another reporting client, pending
// admin review and optional conversion into a new phishing template.
type ReportedMessage struct {
	ID                 int64     `json:"id" gorm:"column:id; primary_key:yes"`
	ReporterEmail      string    `json:"reporter_email" gorm:"column:reporter_email; not null"`
	Subject            string    `json:"subject" gorm:"column:subject"`
	BodyText           string    `json:"body_text" gorm:"column:body_text; sql:type:text"`
	BodyHTML           string    `json:"body_html" gorm:"column:body_html; sql:type:text"`
	Status             string    `json:"status" gorm:"column:status; not null"`
	ConvertedTemplateID int64    `json:"converted_template_id" gorm:"column:converted_template_id"`
	ReviewedBy         string    `json:"reviewed_by" gorm:"column:reviewed_by"`
	CreatedAt          time.Time `json:"created_at" gorm:"column:created_at"`
	ReviewedAt         time.Time `json:"reviewed_at" gorm:"column:reviewed_at"`
}

// TableName specifies the table name for the ReportedMessage model
func (ReportedMessage) TableName() string {
	return "reported_messages"
}

// CreateReportedMessage saves a new reported message with a default
// "pending" status.
func CreateReportedMessage(rm *ReportedMessage) error {
	rm.Status = ReportedMessageStatusPending
	rm.CreatedAt = time.Now().UTC()
	err := db.Save(rm).Error
	if err != nil {
		log.Errorf("error creating reported message: %v", err)
	}
	return err
}

// GetReportedMessages returns all reported messages with the given status.
// Pass an empty string to return all reported messages regardless of status.
func GetReportedMessages(status string) ([]ReportedMessage, error) {
	rms := []ReportedMessage{}
	query := db.Order("created_at desc")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Find(&rms).Error
	if err != nil {
		log.Errorf("error getting reported messages: %v", err)
	}
	return rms, err
}

// GetReportedMessageByID retrieves a single reported message by its primary key.
func GetReportedMessageByID(id int64) (ReportedMessage, error) {
	rm := ReportedMessage{}
	err := db.Where("id = ?", id).First(&rm).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return rm, ErrReportedMessageNotFound
		}
		log.Errorf("error getting reported message by id: %v", err)
	}
	return rm, err
}

// UpdateReportedMessageStatus transitions a reported message to approved or
// rejected, recording who reviewed it and when. If approved, templateID
// should be the ID of the template created from this message (0 otherwise).
func UpdateReportedMessageStatus(id int64, status string, reviewedBy string, templateID int64) error {
	updates := map[string]interface{}{
		"status":               status,
		"reviewed_by":          reviewedBy,
		"reviewed_at":          time.Now().UTC(),
		"converted_template_id": templateID,
	}
	err := db.Model(&ReportedMessage{}).Where("id = ?", id).Updates(updates).Error
	if err != nil {
		log.Errorf("error updating reported message status: %v", err)
	}
	return err
}
```

- [x] **Step 4: Register the new table with gorm's auto-migration**

Find where existing models (e.g. `DeviceFingerprint`, `BehaviorEvent` from `models/enhanced_tracking.go`) are passed to `db.AutoMigrate(...)` inside `models/models.go`'s `Setup`/migration function, and add `&ReportedMessage{}` to that list.

- [x] **Step 5: Run tests to verify they pass**

Run: `go test ./models/ -run TestReportedMessage -v` (or the full `TestCreateAndGetReportedMessage` / `TestGetReportedMessageByIDNotFound` names)
Expected: PASS
Note: per Global Constraints, this may not build locally on a machine without gcc (cgo sqlite3 dependency) — if so, verify via `go vet ./models/` locally and let CI run the actual test.

- [x] **Step 6: Commit**

```bash
git add models/reported_message.go models/reported_message_test.go models/models.go
git commit -m "feat: add ReportedMessage model for real-phish intake

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Task 2: Public intake endpoint + shared email-parsing helper

**Files:**
- Modify: `controllers/phish.go` (add `ReportUnknownHandler`, register route, add rate limiter field)
- Modify: `controllers/api/import.go` (extract reusable parsing function)
- Test: `controllers/phish_test.go`

**Interfaces:**
- Consumes: `models.CreateReportedMessage` (Task 1).
- Produces: `func ParseEmailContent(content string, convertLinks bool) (subject, text, html string, err error)` in package `api` (moved out of the `ImportEmail` handler body so Task 3 can reuse it) — **exported** so `controllers` can call `api.ParseEmailContent`. `POST /report-unknown` on `PhishingServer`, body `{"reporter_email": string, "subject": string, "body_text": string, "body_html": string}`, always responds `204 No Content` on success (mirrors `ReportHandler`'s existing style) or `400`/`429` on failure.

- [x] **Step 1: Extract the shared parser (refactor, no behavior change)**

In `controllers/api/import.go`, pull the body of `ImportEmail` (from `email.NewEmailFromReader` through the `ConvertLinks` goquery rewrite) into:

```go
// ParseEmailContent parses a raw RFC 822 email into its subject, text, and
// HTML parts. When convertLinks is true, all <a href> targets in the HTML
// body are rewritten to "{{.URL}}" so the result is ready to use as a
// phishing template.
func ParseEmailContent(content string, convertLinks bool) (subject, text, html string, err error) {
	e, err := email.NewEmailFromReader(strings.NewReader(content))
	if err != nil {
		return "", "", "", err
	}
	htmlBytes := e.HTML
	if convertLinks {
		d, derr := goquery.NewDocumentFromReader(bytes.NewReader(e.HTML))
		if derr != nil {
			return "", "", "", derr
		}
		d.Find("a").Each(func(i int, a *goquery.Selection) {
			a.SetAttr("href", "{{.URL}}")
		})
		h, herr := d.Html()
		if herr != nil {
			return "", "", "", herr
		}
		htmlBytes = []byte(h)
	}
	return e.Subject, string(e.Text), string(htmlBytes), nil
}
```

Then rewrite `ImportEmail` to call it:

```go
func (as *Server) ImportEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusBadRequest)
		return
	}
	ir := struct {
		Content      string `json:"content"`
		ConvertLinks bool   `json:"convert_links"`
	}{}
	err := json.NewDecoder(r.Body).Decode(&ir)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error decoding JSON Request"}, http.StatusBadRequest)
		return
	}
	subject, text, html, err := ParseEmailContent(ir.Content, ir.ConvertLinks)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}
	JSONResponse(w, emailResponse{Subject: subject, Text: text, HTML: html}, http.StatusOK)
}
```

- [x] **Step 2: Run existing import tests to confirm no regression**

Run: `go test ./controllers/api/ -run TestImportEmail -v`
Expected: PASS (behavior unchanged, only refactored)

- [x] **Step 3: Add the rate limiter field and route to `PhishingServer`**

In `controllers/phish.go`, add a limiter field and wire the new route:

```go
type PhishingServer struct {
	server         *http.Server
	config         config.PhishServer
	contactAddress string
	limiter        *ratelimit.PostLimiter // add this field
}
```

```go
// in NewPhishingServer, before ps.registerRoutes():
ps.limiter = ratelimit.NewPostLimiter()
```

```go
// in registerRoutes():
router.HandleFunc("/report-unknown", mid.Use(ps.ReportUnknownHandler, ps.limiter.Limit)).Methods("POST")
```

Add imports: `"github.com/fir3storm/AwareNow/middleware/ratelimit"` and confirm `mid` (already imported as `"github.com/fir3storm/AwareNow/middleware"`) covers `mid.Use`.

- [x] **Step 4: Write the failing test**

```go
// in controllers/phish_test.go — follow the existing suite's setup pattern
// (check_test.go / gocheck style already used in this file)
func (s *ControllersSuite) TestReportUnknownHandler(c *check.C) {
	body := `{"reporter_email":"alice@example.com","subject":"Urgent: verify your account","body_text":"click here","body_html":"<p>click <a href=\"http://evil.example\">here</a></p>"}`
	req, err := http.NewRequest("POST", "/report-unknown", strings.NewReader(body))
	c.Assert(err, check.Equals, nil)
	req.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	s.phishingServer.ServeHTTP(response, req)
	c.Assert(response.Code, check.Equals, http.StatusNoContent)

	msgs, err := models.GetReportedMessages(models.ReportedMessageStatusPending)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(msgs), check.Equals, 1)
	c.Assert(msgs[0].ReporterEmail, check.Equals, "alice@example.com")
}
```

(Match this to whatever the existing `ControllersSuite` fixture/handler-access pattern is in `controllers/phish_test.go` and `controllers/controllers_test.go` — adjust field/method names to what's actually there rather than inventing new suite plumbing.)

- [x] **Step 5: Run test to verify it fails**

Run: `go test ./controllers/ -run TestReportUnknownHandler -v`
Expected: FAIL — `ReportUnknownHandler` undefined.

- [x] **Step 6: Implement `ReportUnknownHandler`**

```go
// ReportUnknownHandler accepts a report of a real, non-campaign suspicious
// email (one with no AwareNow tracking rid). It stores the report for admin
// review; it does not touch any Result or campaign.
func (ps *PhishingServer) ReportUnknownHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*") // same rationale as ReportHandler: external reporting clients
	var payload struct {
		ReporterEmail string `json:"reporter_email"`
		Subject       string `json:"subject"`
		BodyText      string `json:"body_text"`
		BodyHTML      string `json:"body_html"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if payload.ReporterEmail == "" || (payload.BodyText == "" && payload.BodyHTML == "") {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	rm := &models.ReportedMessage{
		ReporterEmail: payload.ReporterEmail,
		Subject:       payload.Subject,
		BodyText:      payload.BodyText,
		BodyHTML:      payload.BodyHTML,
	}
	if err := models.CreateReportedMessage(rm); err != nil {
		log.Error(err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Add `"encoding/json"` to `controllers/phish.go`'s imports if not already present (it is not, per the file as read for this plan).

- [x] **Step 7: Run tests to verify they pass**

Run: `go test ./controllers/ -run TestReportUnknownHandler -v`
Expected: PASS

- [x] **Step 8: Commit**

```bash
git add controllers/phish.go controllers/api/import.go controllers/phish_test.go
git commit -m "feat: add public intake endpoint for real (non-campaign) phishing reports

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Task 3: Admin review API — list / approve / reject

**Files:**
- Create: `controllers/api/reported_message.go`
- Modify: `controllers/api/server.go` (register routes)
- Test: `controllers/api/reported_message_test.go`

**Interfaces:**
- Consumes: `models.GetReportedMessages`, `models.GetReportedMessageByID`, `models.UpdateReportedMessageStatus` (Task 1); `api.ParseEmailContent` (Task 2); existing `models.PostTemplate` / template creation function (confirm exact signature in `models/template.go` before wiring — it takes a `*models.Template` and a user ID in every other handler in `controllers/api/template.go`, follow that exact pattern).
- Produces: `GET /api/reported-messages/`, `GET /api/reported-messages/{id:[0-9]+}`, `POST /api/reported-messages/{id:[0-9]+}/approve`, `POST /api/reported-messages/{id:[0-9]+}/reject` — all behind the existing `mid.RequireAPIKey` + `mid.RequirePermission(models.PermissionModifyObjects)` used elsewhere in `server.go`.

- [x] **Step 1: Write the failing test for list + approve**

```go
// controllers/api/reported_message_test.go
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fir3storm/AwareNow/models"
)

func TestReportedMessagesApprove(t *testing.T) {
	// follow the existing setup used in api_test.go (setupTest/apiKey helpers)
	rm := &models.ReportedMessage{
		ReporterEmail: "bob@example.com",
		Subject:       "Test",
		BodyHTML:      "<p><a href=\"http://evil.example\">link</a></p>",
	}
	if err := models.CreateReportedMessage(rm); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/reported-messages/"+itoa(rm.ID)+"/approve", strings.NewReader(`{"name":"From report: Test"}`))
	// attach API key header the same way other tests in this package do
	w := httptest.NewRecorder()
	apiServer.ServeHTTP(w, req) // apiServer: reuse whatever package-level test server api_test.go already builds

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, err := models.GetReportedMessageByID(rm.ID)
	if err != nil {
		t.Fatalf("GetReportedMessageByID failed: %v", err)
	}
	if got.Status != models.ReportedMessageStatusApproved {
		t.Fatalf("expected approved, got %s", got.Status)
	}
	if got.ConvertedTemplateID == 0 {
		t.Fatal("expected ConvertedTemplateID to be set")
	}
}
```

Adjust the request-construction/auth boilerplate to match whatever `controllers/api/api_test.go` already sets up for other authenticated-endpoint tests (there is an existing pattern there — do not invent a second one).

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./controllers/api/ -run TestReportedMessagesApprove -v`
Expected: FAIL — route not registered / 404.

- [x] **Step 3: Implement the handlers**

```go
// controllers/api/reported_message.go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	ctx "github.com/fir3storm/AwareNow/context"
	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/gorilla/mux"
)

// ReportedMessages returns all reported messages, optionally filtered by
// the ?status= query parameter.
// GET /api/reported-messages/
func (as *Server) ReportedMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	status := r.URL.Query().Get("status")
	msgs, err := models.GetReportedMessages(status)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error retrieving reported messages"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, msgs, http.StatusOK)
}

// ReportedMessage returns a single reported message by ID.
// GET /api/reported-messages/{id}
func (as *Server) ReportedMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}
	rm, err := models.GetReportedMessageByID(id)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Reported message not found"}, http.StatusNotFound)
		return
	}
	JSONResponse(w, rm, http.StatusOK)
}

// ReportedMessageApprove converts a reported message into a new draft
// template and marks it approved.
// POST /api/reported-messages/{id}/approve
// Body: {"name": "<template name>"}
func (as *Server) ReportedMessageApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}
	rm, err := models.GetReportedMessageByID(id)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Reported message not found"}, http.StatusNotFound)
		return
	}
	if rm.Status != models.ReportedMessageStatusPending {
		JSONResponse(w, models.Response{Success: false, Message: "Reported message already reviewed"}, http.StatusConflict)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		body.Name = "From report: " + rm.Subject
	}

	source := rm.BodyHTML
	convertLinks := rm.BodyHTML != ""
	_, text, html, err := ParseEmailContent(rawEmailFromParts(rm.Subject, rm.BodyText, source), convertLinks)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error converting message to template"}, http.StatusInternalServerError)
		return
	}

	uid := ctx.Get(r, "user_id").(int64)
	tmpl := models.Template{
		UserId: uid,
		Name:   body.Name,
		Subject: rm.Subject,
		Text:   text,
		HTML:   html,
	}
	// Follow whatever models.PostTemplate(&tmpl) / models.Template validation
	// signature controllers/api/template.go already uses for POST /templates/ —
	// call the same function here instead of duplicating validation logic.
	if err := models.PostTemplate(&tmpl); err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}

	reviewedBy := ctx.Get(r, "user").(models.User).Username
	if err := models.UpdateReportedMessageStatus(id, models.ReportedMessageStatusApproved, reviewedBy, tmpl.Id); err != nil {
		log.Error(err)
	}
	JSONResponse(w, tmpl, http.StatusOK)
}

// ReportedMessageReject dismisses a reported message without creating a template.
// POST /api/reported-messages/{id}/reject
func (as *Server) ReportedMessageReject(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}
	reviewedBy := ctx.Get(r, "user").(models.User).Username
	if err := models.UpdateReportedMessageStatus(id, models.ReportedMessageStatusRejected, reviewedBy, 0); err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error updating reported message"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Reported message rejected"}, http.StatusOK)
}
```

`rawEmailFromParts` is a small local helper needed because `ParseEmailContent` takes a raw RFC 822 message, but a `ReportedMessage` stores already-split subject/text/html. Add it in the same file:

```go
// rawEmailFromParts builds a minimal raw RFC 822 message from already-split
// fields so it can be run back through ParseEmailContent's link-rewriting
// logic without duplicating that logic.
func rawEmailFromParts(subject, text, html string) string {
	if html != "" {
		return "Subject: " + subject + "\r\nContent-Type: text/html\r\n\r\n" + html
	}
	return "Subject: " + subject + "\r\nContent-Type: text/plain\r\n\r\n" + text
}
```

**Before writing this file for real:** open `models/template.go` and `controllers/api/template.go` to confirm the exact `Template` struct field names and the exact template-creation function signature (`PostTemplate` is a guess based on the `PostCampaign`/`PostCampaignSMTP` naming convention seen elsewhere in `models/`) — adjust the calls above to match what's actually there rather than what's assumed here.

- [x] **Step 4: Register routes**

In `controllers/api/server.go`, inside `registerRoutes()`, add alongside the other `RequirePermission` routes:

```go
router.HandleFunc("/reported-messages/", mid.Use(as.ReportedMessages, mid.RequirePermission(models.PermissionModifyObjects)))
router.HandleFunc("/reported-messages/{id:[0-9]+}", mid.Use(as.ReportedMessage, mid.RequirePermission(models.PermissionModifyObjects)))
router.HandleFunc("/reported-messages/{id:[0-9]+}/approve", mid.Use(as.ReportedMessageApprove, mid.RequirePermission(models.PermissionModifyObjects))).Methods("POST")
router.HandleFunc("/reported-messages/{id:[0-9]+}/reject", mid.Use(as.ReportedMessageReject, mid.RequirePermission(models.PermissionModifyObjects))).Methods("POST")
```

- [x] **Step 5: Run tests to verify they pass**

Run: `go test ./controllers/api/ -run TestReportedMessages -v`
Expected: PASS

- [x] **Step 6: Commit**

```bash
git add controllers/api/reported_message.go controllers/api/server.go controllers/api/reported_message_test.go
git commit -m "feat: add admin review API for reported phishing messages

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Task 4: Frontend — Reported Messages review page

**Files:**
- Create: `web/src/api/reportedMessages.ts`
- Create: `web/src/pages/ReportedMessages/ReportedMessageList.tsx`
- Modify: wherever the sidebar/route table lives (check `web/src/App.tsx` for the existing route list and nav — follow its exact pattern, e.g. how `Groups`/`Templates` are wired)
- Test: `web/src/pages/ReportedMessages/ReportedMessageList.test.tsx`

**Interfaces:**
- Consumes: `GET/POST /api/reported-messages/...` (Task 3). Follow `web/src/api/client.ts`'s existing axios instance and `web/src/api/templates.ts`'s existing call style exactly — read both before writing this task's code.
- Produces: a route (path TBD to match existing convention, e.g. `/reported-messages`) reachable from the nav.

- [x] **Step 1: Read the existing patterns first**

Open `web/src/api/templates.ts`, `web/src/pages/Templates/TemplateList.tsx`, and `web/src/App.tsx` in full. This task's list/detail/approve/reject page should structurally mirror `TemplateList.tsx` (same data-fetching hook style — likely `@tanstack/react-query`, same table component, same loading/error states) rather than introducing a new pattern.

- [x] **Step 2: Write the API client**

```ts
// web/src/api/reportedMessages.ts
import { apiClient } from './client'; // match the actual export name in client.ts

export type ReportedMessageStatus = 'pending' | 'approved' | 'rejected';

export interface ReportedMessage {
  id: number;
  reporter_email: string;
  subject: string;
  body_text: string;
  body_html: string;
  status: ReportedMessageStatus;
  converted_template_id: number;
  reviewed_by: string;
  created_at: string;
  reviewed_at: string;
}

export async function listReportedMessages(status?: ReportedMessageStatus) {
  const params = status ? { status } : undefined;
  const { data } = await apiClient.get<ReportedMessage[]>('/reported-messages/', { params });
  return data;
}

export async function approveReportedMessage(id: number, name: string) {
  const { data } = await apiClient.post(`/reported-messages/${id}/approve`, { name });
  return data;
}

export async function rejectReportedMessage(id: number) {
  const { data } = await apiClient.post(`/reported-messages/${id}/reject`);
  return data;
}
```

(Adjust the exact axios wrapper import/usage to whatever `web/src/api/templates.ts` actually does — this is illustrative of the shape, not a guaranteed match to the real client helper's name.)

- [x] **Step 3: Write the failing component test**

```tsx
// web/src/pages/ReportedMessages/ReportedMessageList.test.tsx
import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ReportedMessageList } from './ReportedMessageList';
import * as api from '../../api/reportedMessages';

vi.spyOn(api, 'listReportedMessages').mockResolvedValue([
  {
    id: 1,
    reporter_email: 'alice@example.com',
    subject: 'Urgent: verify your account',
    body_text: '',
    body_html: '',
    status: 'pending',
    converted_template_id: 0,
    reviewed_by: '',
    created_at: '2026-09-05T00:00:00Z',
    reviewed_at: '',
  },
]);

describe('ReportedMessageList', () => {
  it('renders a pending reported message', async () => {
    render(<ReportedMessageList />);
    expect(await screen.findByText('Urgent: verify your account')).toBeInTheDocument();
    expect(screen.getByText('alice@example.com')).toBeInTheDocument();
  });
});
```

- [x] **Step 4: Run test to verify it fails**

Run: `cd web && npx vitest run src/pages/ReportedMessages/ReportedMessageList.test.tsx`
Expected: FAIL — module does not exist.

- [x] **Step 5: Implement the component**

Build `ReportedMessageList.tsx` following `TemplateList.tsx`'s exact structure (query hook, table, loading/error states), with columns: reporter email, subject, reported date, status, and Approve/Reject action buttons that call `approveReportedMessage`/`rejectReportedMessage` and invalidate the list query on success. Do not invent a new table/loading pattern — copy the established one.

- [x] **Step 6: Wire the route and nav entry**

Add the route/nav entry in `web/src/App.tsx` following the exact pattern used for the existing pages there.

- [x] **Step 7: Run tests to verify they pass**

Run: `cd web && npx vitest run src/pages/ReportedMessages/ReportedMessageList.test.tsx`
Expected: PASS

- [x] **Step 8: Commit**

```bash
git add web/src/api/reportedMessages.ts web/src/pages/ReportedMessages/ web/src/App.tsx
git commit -m "feat: add Reported Messages review page to web UI

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Task 5: Outlook add-in — one-click report button (independent of Tasks 1-4)

**Files:**
- Create: `addins/outlook-report-button/manifest.xml`
- Create: `addins/outlook-report-button/commands.html`
- Create: `addins/outlook-report-button/src/extractRid.js`
- Create: `addins/outlook-report-button/src/commands.js`
- Create: `addins/outlook-report-button/src/extractRid.test.js`
- Create: `addins/outlook-report-button/package.json`
- Create: `addins/outlook-report-button/README.md`

**Interfaces:**
- Consumes: the **existing** `GET/POST /report?rid=<7-char-id>` endpoint (`controllers/phish.go`'s `ReportHandler`, already implemented, already CORS-open) for known campaign emails; the new `POST /report-unknown` endpoint (Task 2) for anything else.
- Produces: a side-loadable Office Add-in manifest an admin can deploy org-wide via Microsoft 365 admin center (Integrated Apps) or side-load individually for testing.

- [ ] **Step 1: Write the failing unit test for rid extraction**

```js
// addins/outlook-report-button/src/extractRid.test.js
import { describe, it, expect } from 'vitest';
import { extractRid } from './extractRid.js';

describe('extractRid', () => {
  it('extracts a 7-char rid from a query-string-style link', () => {
    const body = 'Please review your invoice: https://itsupport.insec.in/invoice?rid=AbC1234';
    expect(extractRid(body)).toBe('AbC1234');
  });

  it('extracts a rid joined with &', () => {
    const body = 'Click here: https://example.com/x?foo=bar&rid=Zz98765';
    expect(extractRid(body)).toBe('Zz98765');
  });

  it('returns null when no rid is present', () => {
    const body = 'This is a completely unrelated real email with no tracking link.';
    expect(extractRid(body)).toBeNull();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd addins/outlook-report-button && npx vitest run src/extractRid.test.js`
Expected: FAIL — module does not exist.

- [ ] **Step 3: Implement `extractRid`**

```js
// addins/outlook-report-button/src/extractRid.js
// Mirrors the rid pattern imap/monitor.go already looks for
// (7-character alphanumeric AwareNow tracking ID, ?rid= or &rid=).
const RID_PATTERN = /[?&]rid=([A-Za-z0-9]{7})\b/;

export function extractRid(text) {
  if (!text) return null;
  const match = RID_PATTERN.exec(text);
  return match ? match[1] : null;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd addins/outlook-report-button && npx vitest run src/extractRid.test.js`
Expected: PASS

- [ ] **Step 5: Add minimal package.json for the test runner**

```json
{
  "name": "@awarenow/outlook-report-button",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "test": "vitest run"
  },
  "devDependencies": {
    "vitest": "^3.2.4"
  }
}
```

- [ ] **Step 6: Write `commands.js`, the add-in's actual behavior**

```js
// addins/outlook-report-button/src/commands.js
import { extractRid } from './extractRid.js';

// Admin sets this once per deployment (see README) — the org's AwareNow
// phishing-server base URL, e.g. "https://itsupport.insec.in:8082".
const SERVER_URL_SETTING = 'awarenowServerUrl';

Office.onReady(() => {
  // Office.js requires this call before any Office API is used, even
  // though this add-in has no UI beyond a ribbon button.
});

function getServerUrl() {
  return Office.context.roamingSettings.get(SERVER_URL_SETTING) || '';
}

function reportPhishing(event) {
  const item = Office.context.mailbox.item;
  const serverUrl = getServerUrl();

  if (!serverUrl) {
    Office.context.mailbox.item.notificationMessages.replaceAsync('awarenow-config', {
      type: 'errorMessage',
      message: 'AwareNow server URL is not configured. Ask your admin to set it via Outlook add-in settings.',
    });
    event.completed();
    return;
  }

  item.body.getAsync(Office.CoercionType.Text, (bodyResult) => {
    const bodyText = bodyResult.status === Office.AsyncResultStatus.Succeeded ? bodyResult.value : '';
    const rid = extractRid(bodyText);

    const done = (message) => {
      Office.context.mailbox.item.notificationMessages.replaceAsync('awarenow-report', {
        type: 'informationalMessage',
        message,
        icon: 'icon1',
        persistent: false,
      });
      event.completed();
    };

    if (rid) {
      fetch(`${serverUrl}/report?rid=${encodeURIComponent(rid)}`, { method: 'POST', mode: 'cors' })
        .then(() => done('Thanks — this simulated phishing email was reported.'))
        .catch(() => done('Could not reach the AwareNow server. Try again later.'));
      return;
    }

    item.subject.getAsync((subjectResult) => {
      const subject = subjectResult.status === Office.AsyncResultStatus.Succeeded ? subjectResult.value : '';
      fetch(`${serverUrl}/report-unknown`, {
        method: 'POST',
        mode: 'cors',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          reporter_email: Office.context.mailbox.userProfile.emailAddress,
          subject,
          body_text: bodyText,
        }),
      })
        .then(() => done('Thanks — this email was reported to your security team for review.'))
        .catch(() => done('Could not reach the AwareNow server. Try again later.'));
    });
  });
}

// Register the function so the ribbon button (declared in manifest.xml) can invoke it.
Office.actions = Office.actions || {};
Office.actions.associate('reportPhishing', reportPhishing);
```

- [ ] **Step 7: Write `commands.html`, the function-file host page**

```html
<!doctype html>
<html>
<head>
  <meta charset="UTF-8" />
  <title>AwareNow Report Button Commands</title>
  <script src="https://appsforoffice.microsoft.com/lib/1/hosted/office.js"></script>
  <script type="module" src="./src/commands.js"></script>
</head>
<body></body>
</html>
```

- [ ] **Step 8: Write `manifest.xml`**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<OfficeApp
    xmlns="http://schemas.microsoft.com/office/appforoffice/1.1"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
    xmlns:bt="http://schemas.microsoft.com/office/officeappbasictypes/1.0"
    xmlns:mailappor="http://schemas.microsoft.com/office/mailappversionoverrides/1.0"
    xsi:type="MailApp">
  <Id>a1b2c3d4-e5f6-47a8-9b0c-1d2e3f4a5b6c</Id>
  <Version>1.0.0.0</Version>
  <ProviderName>AwareNow</ProviderName>
  <DefaultLocale>en-US</DefaultLocale>
  <DisplayName DefaultValue="Report Phishing" />
  <Description DefaultValue="Report a suspicious or simulated phishing email to AwareNow with one click." />
  <IconUrl DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST/addins/outlook-report-button/icon-32.png" />
  <HighResolutionIconUrl DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST/addins/outlook-report-button/icon-80.png" />
  <SupportUrl DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST" />
  <AppDomains>
    <AppDomain>https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST</AppDomain>
  </AppDomains>
  <Hosts>
    <Host Name="Mailbox" />
  </Hosts>
  <Requirements>
    <Sets>
      <Set Name="Mailbox" MinVersion="1.5" />
    </Sets>
  </Requirements>
  <FormSettings>
    <Form xsi:type="ItemRead">
      <DesktopSettings>
        <SourceLocation DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST/addins/outlook-report-button/commands.html" />
      </DesktopSettings>
    </Form>
  </FormSettings>
  <Permissions>ReadItem</Permissions>
  <Rule xsi:type="RuleCollection" Mode="Or">
    <Rule xsi:type="ItemIs" ItemType="Message" FormType="Read" />
  </Rule>
  <VersionOverrides xmlns="http://schemas.microsoft.com/office/mailappversionoverrides" xsi:type="VersionOverridesV1_0">
    <Requirements>
      <bt:Sets DefaultMinVersion="1.3">
        <bt:Set Name="Mailbox" />
      </bt:Sets>
    </Requirements>
    <Hosts>
      <Host xsi:type="MailHost">
        <DesktopFormFactor>
          <FunctionFile resid="commands.url" />
          <ExtensionPoint xsi:type="MessageReadCommandSurface">
            <OfficeTab id="TabDefault">
              <Group id="awarenowGroup">
                <Label resid="groupLabel" />
                <Control xsi:type="Button" id="reportPhishingButton">
                  <Label resid="buttonLabel" />
                  <Supertip>
                    <Title resid="buttonLabel" />
                    <Description resid="buttonTooltip" />
                  </Supertip>
                  <Icon>
                    <bt:Image size="16" resid="icon16" />
                    <bt:Image size="32" resid="icon32" />
                    <bt:Image size="80" resid="icon80" />
                  </Icon>
                  <Action xsi:type="ExecuteFunction">
                    <FunctionName>reportPhishing</FunctionName>
                  </Action>
                </Control>
              </Group>
            </OfficeTab>
          </ExtensionPoint>
        </DesktopFormFactor>
      </Host>
    </Hosts>
    <Resources>
      <bt:Images>
        <bt:Image id="icon16" DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST/addins/outlook-report-button/icon-16.png" />
        <bt:Image id="icon32" DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST/addins/outlook-report-button/icon-32.png" />
        <bt:Image id="icon80" DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST/addins/outlook-report-button/icon-80.png" />
      </bt:Images>
      <bt:Urls>
        <bt:Url id="commands.url" DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST/addins/outlook-report-button/commands.html" />
      </bt:Urls>
      <bt:ShortStrings>
        <bt:String id="groupLabel" DefaultValue="AwareNow" />
        <bt:String id="buttonLabel" DefaultValue="Report Phishing" />
      </bt:ShortStrings>
      <bt:LongStrings>
        <bt:String id="buttonTooltip" DefaultValue="Report this email as suspicious or as a simulated phishing test." />
      </bt:LongStrings>
    </Resources>
  </VersionOverrides>
</OfficeApp>
```

- [ ] **Step 9: Write the README covering the two things this plan cannot automate**

```markdown
# AwareNow Outlook Report Button

## Deploy checklist (manual — cannot be scripted from this repo)

1. Replace every `REPLACE_WITH_YOUR_DEPLOYMENT_HOST` in `manifest.xml` with
   this deployment's actual public HTTPS host serving this add-in's static
   files (commands.html, src/, icons). Serve them from the admin server's
   existing static-file path or a small dedicated host — this add-in has
   no server-side component beyond the two endpoints it calls.
2. Validate the manifest: `npx office-addin-manifest validate manifest.xml`
3. Side-load for testing: Outlook desktop → Get Add-ins → My add-ins →
   Add a custom add-in → Add from file → select `manifest.xml`.
4. For org-wide rollout: Microsoft 365 admin center → Settings →
   Integrated apps → Upload custom apps.
5. Each end user (or an admin, via Office.js roaming settings pushed at
   deploy time) must set the AwareNow server URL once — there is
   currently no settings UI for this in v1; the quickest path is a
   one-time `Office.context.roamingSettings.set('awarenowServerUrl', '<url>')`
   run from the browser console while the add-in is loaded, or add a
   proper settings dialog as a fast-follow.
```

- [ ] **Step 10: Commit**

```bash
git add addins/outlook-report-button/
git commit -m "feat: add Outlook one-click report button add-in

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Task 6: PDF analytics export (independent)

**Files:**
- Modify: `go.mod`, `go.sum` (add `github.com/go-pdf/fpdf`)
- Create: `controllers/api/export_pdf.go`
- Modify: `controllers/api/analytics.go` (`ExportAnalytics`'s `case "pdf"`)
- Test: `controllers/api/export_pdf_test.go`

**Interfaces:**
- Consumes: `models.GetAnalyticsOverview`, `models.GetOverallTimeline`, `models.GetDepartmentStats`, `models.GetRiskScore` (all already exist, used identically in the existing `exportCSV`/`generateCSVFromAnalytics` in `controllers/api/analytics.go` — reuse those exact calls).
- Produces: `func exportPDF(w http.ResponseWriter, r *http.Request, uid int64)` matching the existing `exportCSV`/`exportJSON` signature pattern.

- [ ] **Step 1: Add the dependency**

```bash
go get github.com/go-pdf/fpdf@latest
```

- [ ] **Step 2: Write the failing test**

```go
// controllers/api/export_pdf_test.go
package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExportAnalyticsPDF(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/analytics/export?format=pdf", nil)
	// attach auth the same way other tests in this package do
	w := httptest.NewRecorder()
	apiServer.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("expected Content-Type application/pdf, got %s", ct)
	}
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF")) {
		t.Fatal("expected response body to start with the PDF magic bytes")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./controllers/api/ -run TestExportAnalyticsPDF -v`
Expected: FAIL — still returns 501 per the current `case "pdf", "xlsx":` branch.

- [ ] **Step 4: Implement `exportPDF`**

```go
// controllers/api/export_pdf.go
package api

import (
	"fmt"
	"net/http"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/go-pdf/fpdf"
)

func exportPDF(w http.ResponseWriter, r *http.Request, uid int64) {
	overview, err := models.GetAnalyticsOverview(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting data"}, http.StatusInternalServerError)
		return
	}
	depts, err := models.GetDepartmentStats(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting department stats"}, http.StatusInternalServerError)
		return
	}
	risk, err := models.GetRiskScore(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting risk score"}, http.StatusInternalServerError)
		return
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "AwareNow Analytics Report", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, "Generated "+time.Now().UTC().Format(time.RFC1123), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Overview", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	rows := [][2]string{
		{"Total Campaigns", fmt.Sprintf("%d", overview.TotalCampaigns)},
		{"Emails Sent", fmt.Sprintf("%d", overview.EmailsSent)},
		{"Open Rate", fmt.Sprintf("%.2f%%", overview.OpenRate)},
		{"Click Rate", fmt.Sprintf("%.2f%%", overview.ClickRate)},
		{"Submit Rate", fmt.Sprintf("%.2f%%", overview.SubmitRate)},
		{"Report Rate", fmt.Sprintf("%.2f%%", overview.ReportRate)},
		{"Risk Score", fmt.Sprintf("%d (%s)", risk.Score, risk.Level)},
	}
	for _, row := range rows {
		pdf.CellFormat(60, 6, row[0], "", 0, "L", false, 0, "")
		pdf.CellFormat(0, 6, row[1], "", 1, "L", false, 0, "")
	}
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Department Statistics", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(70, 6, "Department", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 6, "Users", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 6, "Click Rate", "1", 0, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Submit Rate", "1", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	for _, d := range depts {
		pdf.CellFormat(70, 6, d.Department, "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 6, fmt.Sprintf("%d", d.UsersCount), "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 6, fmt.Sprintf("%.2f%%", d.ClickRate), "1", 0, "L", false, 0, "")
		pdf.CellFormat(0, 6, fmt.Sprintf("%.2f%%", d.SubmitRate), "1", 1, "L", false, 0, "")
	}

	filename := fmt.Sprintf("analytics_export_%s.pdf", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	if err := pdf.Output(w); err != nil {
		log.Errorf("error writing PDF output: %v", err)
	}
}
```

- [ ] **Step 5: Wire it into `ExportAnalytics`**

In `controllers/api/analytics.go`, change:

```go
case "pdf", "xlsx":
	JSONResponse(w, models.Response{Success: false, Message: fmt.Sprintf("Format '%s' export not yet implemented. Use 'csv' or 'json'.", format)}, http.StatusNotImplemented)
```

to:

```go
case "pdf":
	exportPDF(w, r, uid)
case "xlsx":
	exportXLSX(w, r, uid) // implemented in Task 7 — until that task lands, leave this line out and keep xlsx routed to the NotImplemented branch
```

(If Task 7 hasn't shipped yet when this task ships, keep `"xlsx"` in the not-implemented branch and only pull `"pdf"` out of it — do not reference `exportXLSX` before it exists.)

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./controllers/api/ -run TestExportAnalyticsPDF -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum controllers/api/export_pdf.go controllers/api/analytics.go controllers/api/export_pdf_test.go
git commit -m "feat: implement PDF analytics export

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Task 7: XLSX analytics export (independent)

**Files:**
- Modify: `go.mod`, `go.sum` (add `github.com/xuri/excelize/v2`)
- Create: `controllers/api/export_xlsx.go`
- Modify: `controllers/api/analytics.go` (`ExportAnalytics`'s `case "xlsx"`, same edit point as Task 6 — coordinate order of landing with whichever of Task 6/7 merges second)
- Test: `controllers/api/export_xlsx_test.go`

**Interfaces:**
- Consumes: same analytics model functions as Task 6, plus `models.GetOverallTimeline`.
- Produces: `func exportXLSX(w http.ResponseWriter, r *http.Request, uid int64)`.

- [ ] **Step 1: Add the dependency**

```bash
go get github.com/xuri/excelize/v2@latest
```

- [ ] **Step 2: Write the failing test**

```go
// controllers/api/export_xlsx_test.go
package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExportAnalyticsXLSX(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/analytics/export?format=xlsx", nil)
	w := httptest.NewRecorder()
	apiServer.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	wantCT := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if ct := w.Header().Get("Content-Type"); ct != wantCT {
		t.Fatalf("expected Content-Type %s, got %s", wantCT, ct)
	}
	// XLSX files are zip archives; PK\x03\x04 is the zip local-file-header magic.
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("PK\x03\x04")) {
		t.Fatal("expected response body to start with the zip/xlsx magic bytes")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./controllers/api/ -run TestExportAnalyticsXLSX -v`
Expected: FAIL

- [ ] **Step 4: Implement `exportXLSX`**

```go
// controllers/api/export_xlsx.go
package api

import (
	"fmt"
	"net/http"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/xuri/excelize/v2"
)

func exportXLSX(w http.ResponseWriter, r *http.Request, uid int64) {
	overview, err := models.GetAnalyticsOverview(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting data"}, http.StatusInternalServerError)
		return
	}
	timeline, err := models.GetOverallTimeline(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting timeline"}, http.StatusInternalServerError)
		return
	}
	depts, err := models.GetDepartmentStats(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting department stats"}, http.StatusInternalServerError)
		return
	}
	risk, err := models.GetRiskScore(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting risk score"}, http.StatusInternalServerError)
		return
	}

	f := excelize.NewFile()
	defer f.Close()

	const overviewSheet = "Overview"
	f.SetSheetName("Sheet1", overviewSheet)
	overviewRows := [][2]interface{}{
		{"Total Campaigns", overview.TotalCampaigns},
		{"Emails Sent", overview.EmailsSent},
		{"Open Rate (%)", overview.OpenRate},
		{"Click Rate (%)", overview.ClickRate},
		{"Submit Rate (%)", overview.SubmitRate},
		{"Report Rate (%)", overview.ReportRate},
		{"Risk Score", risk.Score},
		{"Risk Level", risk.Level},
	}
	for i, row := range overviewRows {
		f.SetCellValue(overviewSheet, fmt.Sprintf("A%d", i+1), row[0])
		f.SetCellValue(overviewSheet, fmt.Sprintf("B%d", i+1), row[1])
	}

	const timelineSheet = "Timeline"
	f.NewSheet(timelineSheet)
	f.SetCellValue(timelineSheet, "A1", "Date")
	f.SetCellValue(timelineSheet, "B1", "Opens")
	f.SetCellValue(timelineSheet, "C1", "Clicks")
	f.SetCellValue(timelineSheet, "D1", "Submits")
	for i, t := range timeline {
		row := i + 2
		f.SetCellValue(timelineSheet, fmt.Sprintf("A%d", row), t.Date)
		f.SetCellValue(timelineSheet, fmt.Sprintf("B%d", row), t.Opens)
		f.SetCellValue(timelineSheet, fmt.Sprintf("C%d", row), t.Clicks)
		f.SetCellValue(timelineSheet, fmt.Sprintf("D%d", row), t.Submits)
	}

	const deptSheet = "Departments"
	f.NewSheet(deptSheet)
	f.SetCellValue(deptSheet, "A1", "Department")
	f.SetCellValue(deptSheet, "B1", "Users")
	f.SetCellValue(deptSheet, "C1", "Click Rate (%)")
	f.SetCellValue(deptSheet, "D1", "Submit Rate (%)")
	for i, d := range depts {
		row := i + 2
		f.SetCellValue(deptSheet, fmt.Sprintf("A%d", row), d.Department)
		f.SetCellValue(deptSheet, fmt.Sprintf("B%d", row), d.UsersCount)
		f.SetCellValue(deptSheet, fmt.Sprintf("C%d", row), d.ClickRate)
		f.SetCellValue(deptSheet, fmt.Sprintf("D%d", row), d.SubmitRate)
	}

	f.SetActiveSheet(0)

	filename := fmt.Sprintf("analytics_export_%s.xlsx", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	if err := f.Write(w); err != nil {
		log.Errorf("error writing XLSX output: %v", err)
	}
}
```

- [ ] **Step 5: Wire it into `ExportAnalytics`**

Same edit point as Task 6, Step 5 — add the `case "xlsx": exportXLSX(w, r, uid)` arm. If Task 6 already landed and left `case "pdf": exportPDF(...)` in place with `"xlsx"` still falling into the not-implemented branch, split that branch's `case` line to remove `"xlsx"` from it.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./controllers/api/ -run TestExportAnalyticsXLSX -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum controllers/api/export_xlsx.go controllers/api/analytics.go controllers/api/export_xlsx_test.go
git commit -m "feat: implement XLSX analytics export

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Self-Review Notes (from initial authoring, 2026-09-05)

- **Spec coverage:** Task 5 covers roadmap item A (client half — the server half already existed pre-plan and is documented as such rather than re-built). Tasks 1-4 cover item B (report → template). Tasks 6-7 cover item C (PDF/XLSX export). All three Phase 1 items from the 2026-09-05 roadmap discussion are covered.
- **Known unknowns flagged inline rather than guessed silently:** the exact `models.Template`/`PostTemplate` signature (Task 3), the exact `setupTest`/`tearDown` test fixture names (Task 1), the exact `ControllersSuite` test plumbing (Task 2), and the exact axios client export shape (Task 4) are all called out as "confirm before writing" rather than invented — the assigned implementer (subagent or otherwise) must read the named files first.
- **Task 5 has two manual, non-scriptable steps** (hosting the static files at a real HTTPS URL, and the org rolling the manifest out via M365 admin center or side-loading) — these are called out explicitly in the add-in's own README rather than glossed over, since no amount of code changes this repo's engine makes can automate a Microsoft 365 tenant admin action.
- **Tasks 6 and 7 share one edit point** in `analytics.go`'s `ExportAnalytics` switch — flagged explicitly in both tasks so whichever lands second doesn't silently clobber the other's case arm.
