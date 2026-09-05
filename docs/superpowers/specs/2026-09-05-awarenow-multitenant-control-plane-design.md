# AwareNow Multi-Tenant Control Plane Design

## Purpose

Evolve AwareNow into a polished, multi-tenant phishing-awareness platform without replacing its proven Go/Gophish-compatible campaign engine. This first increment establishes the control-plane contracts, safety boundary, tenant lifecycle foundation, engine control boundary, event pipeline, deployment blueprint, and dashboard shell needed for later feature migration.

## Product boundary

- AwareNow is the sole product name and this repository is its canonical source.
- The existing Go engine remains responsible for campaign execution, mail delivery, landing-page serving, tracking, and campaign results.
- A new TypeScript/Express control plane owns tenancy, operator RBAC, provisioning state, policy enforcement, engine routing, audit records, and cross-tenant aggregate analytics.
- The existing React application becomes the starting point for the AwareNow control-plane user experience; the first increment adds an authenticated tenant-aware dashboard shell rather than duplicating the UI.
- PhishSentinel code is a reference source for behavior and UX ideas. No wholesale source-tree copy or runtime dependency on `D:\My Softwares\Phishing Awareness Tool` is permitted.

## Tenant isolation model

Every tenant receives a separately provisioned engine instance, database, worker identity, delivery credentials, and campaign/tracking domain routing. The control plane stores only the tenant's engine base URL and a separately managed control credential reference; it never shares an engine database or delivery credential across tenants.

The control plane PostgreSQL database is a management plane, not a campaign execution database. Tenant-scoped queries must require a tenant ID derived from authenticated session claims, never from an untrusted request body or query parameter. Platform administrators are explicitly scoped by a platform role and their actions must be auditable.

## Safety and data policy

This product is for authorized security-awareness simulations only.

- Landing-page submission values are never accepted by the control plane and are never placed in events, logs, analytics, queues, or audit records.
- Engine events may contain only an opaque result ID, tenant ID, campaign ID, recipient pseudonym or opaque recipient ID, event type, event time, and non-sensitive field metadata such as count and field names.
- The allowed event types for this increment are `email_sent`, `email_opened`, `link_clicked`, `landing_page_viewed`, `form_submitted`, `report_submitted`, `training_completed`, and `campaign_completed`.
- Control-plane credentials and engine control secrets must come from environment configuration or an external secret provider reference. They must be redacted from logs and API responses.
- All provisioning, policy changes, and privileged engine actions create immutable audit entries.

## First-increment interfaces

### Control-plane HTTP API

All control-plane routes are versioned under `/api/v1`.

| Route | Role | Result |
| --- | --- | --- |
| `GET /api/v1/health` | anonymous | service liveness only |
| `GET /api/v1/tenants/current` | tenant member | tenant identity and lifecycle status |
| `POST /api/v1/tenants` | platform admin | creates a `provisioning` tenant record and audit entry |
| `POST /api/v1/tenants/:tenantId/provision` | platform admin | requests idempotent isolated-engine provisioning |
| `POST /api/v1/events/engine` | tenant engine credential | validates a safe event and stores an aggregate-safe event record |
| `GET /api/v1/analytics/overview` | tenant member | returns only current-tenant aggregate totals |

Authentication integration is represented by a `TenantPrincipal` contract in this increment. Production IdP/session integration comes after the interface and tenant scoping tests are established.

### Engine control API

The Go engine will expose a private, control-plane-only API under `/api/v1/control`:

| Route | Method | Purpose |
| --- | --- | --- |
| `/health` | `GET` | reports engine readiness and engine version |
| `/campaigns` | `GET` | lists safe campaign summaries |
| `/campaigns/:id/stop` | `POST` | stops an engine campaign by numeric ID |

The adapter must use a distinct bearer token from the engine's ordinary administrator session. The control routes must not return recipient addresses, templates, credential values, or captured form values.

### Safe engine event envelope

```ts
type SafeEngineEvent = {
  eventId: string;
  tenantId: string;
  campaignId: string;
  recipientRef: string;
  type:
    | 'email_sent'
    | 'email_opened'
    | 'link_clicked'
    | 'landing_page_viewed'
    | 'form_submitted'
    | 'report_submitted'
    | 'training_completed'
    | 'campaign_completed';
  occurredAt: string;
  fieldNames?: string[];
  fieldCount?: number;
};
```

An event validator rejects unknown keys, credential-like keys (`password`, `secret`, `token`, `credential`, `value`), unsafe event types, an event whose tenant differs from the authenticated engine tenant, and malformed timestamps/identifiers.

## Repository layout after this increment

```text
control-plane/                 # Express management plane
  prisma/schema.prisma         # management-plane data model
  src/config/                  # validated runtime settings
  src/security/                # safe event and secret-redaction policy
  src/tenancy/                 # principal and tenant-lifecycle rules
  src/events/                  # validated safe-event intake and aggregates
  src/engine/                  # private Go-engine adapter
  src/provisioning/            # idempotent tenant provisioning service
deploy/tenant-engine/          # isolated engine compose/templates
controllers/control/           # Go engine private control endpoints
web/src/control-plane/         # tenant-aware dashboard components
docs/architecture/             # versioned inter-service contracts
```

## Definition of done for the foundation increment

1. The existing React build, lint workflow, and GitHub Actions frontend check are reliable.
2. The repository contains a buildable Express control-plane service with health, tenant, event, and analytics route wiring.
3. The PostgreSQL model represents tenants, memberships, engine instances, safe events, aggregate metrics, and immutable audit entries.
4. Unit tests prove tenant scope cannot be overridden and unsafe event fields are rejected.
5. The Go engine's private control handlers authenticate a control bearer token and redact output to safe campaign summaries.
6. Deployment templates demonstrate one isolated engine/database/network per tenant and do not include real credentials.
7. The React shell shows current tenant context, lifecycle state, and aggregate awareness metrics without embedding cross-tenant data.
8. Local and CI documentation tells an operator how to run checks without deploying or sending campaigns.

## Explicitly deferred

SSO/SCIM implementation, a production secret manager, actual container orchestration, tenant domain automation, SMTP/Graph/Gmail connectors, AI scenario generation, SIEM/LMS adapters, scheduled reporting, mobile/SMS vectors, and migration of historical PhishSentinel records are intentionally deferred. Interfaces established here must not preclude them.
