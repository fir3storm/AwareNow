# AwareNow control-plane contracts

This document is the canonical contract between the TypeScript control plane,
the private Go engine control API, and tenant/event domain services. All
timestamps are ISO 8601 UTC strings and all identifiers are opaque strings
unless a contract states otherwise.

## TypeScript contracts

```ts
export type TenantLifecycle =
  | 'PROVISIONING'
  | 'ACTIVE'
  | 'SUSPENDED'
  | 'FAILED';

export type TenantPrincipal = {
  subjectId: string;
  role: 'platform_admin' | 'tenant_member' | 'tenant_engine';
  tenantId: string | null;
  engineTenantId: string | null;
};

export type SafeEngineEvent = {
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

export type EngineHealth = {
  ready: boolean;
  version: string;
};

export type SafeCampaignSummary = {
  id: number;
  name: string;
  status: string;
  created_at: string;
  launch_date: string | null;
  result_count: number;
};

export type ProvisioningRequest = {
  tenantId: string;
  idempotencyKey: string;
};
```

`TenantPrincipal` is authentication-derived data. A tenant member has its
tenant in `tenantId`; an engine credential has its bound tenant in
`engineTenantId`; a platform administrator without an explicitly selected
tenant has `tenantId: null`.

## HTTP acceptance examples

### Accepted safe engine event

```http
POST /api/v1/events/engine HTTP/1.1
Authorization: Bearer <tenant-engine-control-token>
Content-Type: application/json

{
  "eventId": "evt_01J9A6K4X8ZP7Q2M3N5R",
  "tenantId": "tenant_acme",
  "campaignId": "campaign_42",
  "recipientRef": "recipient_8de9a1",
  "type": "link_clicked",
  "occurredAt": "2026-09-05T09:30:00.000Z"
}
```

```http
HTTP/1.1 202 Accepted
Content-Type: application/json

{
  "eventId": "evt_01J9A6K4X8ZP7Q2M3N5R",
  "status": "accepted"
}
```

The engine credential for this request must be bound to `tenant_acme`. The
control plane records only the safe event fields and aggregate-safe metadata.

### Rejected credential-bearing event

```http
POST /api/v1/events/engine HTTP/1.1
Authorization: Bearer <tenant-engine-control-token>
Content-Type: application/json

{
  "eventId": "evt_unsafe_01",
  "tenantId": "tenant_acme",
  "campaignId": "campaign_42",
  "recipientRef": "recipient_8de9a1",
  "type": "form_submitted",
  "occurredAt": "2026-09-05T09:31:00.000Z",
  "password": "must-not-be-accepted"
}
```

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "unsafe_event"
}
```

The rejected request body, including `password`, is neither stored nor written
to logs, queues, analytics, or audit records.

### Private engine health

```http
GET /api/v1/control/health HTTP/1.1
Authorization: Bearer <awarenow-control-token>
```

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "ready": true,
  "version": "1.0.0"
}
```

### Private safe campaign summary

```http
GET /api/v1/control/campaigns HTTP/1.1
Authorization: Bearer <awarenow-control-token>
```

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "campaigns": [
    {
      "id": 42,
      "name": "Q3 security awareness simulation",
      "status": "completed",
      "created_at": "2026-09-01T08:00:00.000Z",
      "launch_date": "2026-09-03T09:00:00.000Z",
      "result_count": 126
    }
  ]
}
```

## Trust boundaries

- Tenant scope is derived only from an authenticated `TenantPrincipal`. A
  caller-supplied `tenantId` never selects or overrides query, storage, or
  analytics scope. Event intake additionally requires the payload `tenantId`
  to equal the authenticated engine tenant.
- `SafeEngineEvent` is an allowlist. Unknown keys, unsafe event types,
  malformed identifiers or timestamps, and case-insensitive credential-like
  keys (`password`, `secret`, `token`, `credential`, and `value`) are rejected.
  Form submission values are never accepted; only `fieldNames` and
  `fieldCount` may describe a submission.
- Engine control routes are private control-plane-to-engine routes. They use a
  distinct bearer token, `AWARENOW_CONTROL_TOKEN`, rather than an ordinary
  engine administrator session.
- Tokens, credential references, authorization values, and other secrets are
  redacted from logs and API responses. The control plane stores a managed
  credential reference, never the corresponding secret value.
- The engine control API returns only `EngineHealth` and
  `SafeCampaignSummary` data. It never returns recipient addresses, templates,
  captured form values, or credential values.
- Provisioning and privileged engine actions require the appropriate
  authenticated principal and create immutable audit entries. Provisioning is
  idempotent by `ProvisioningRequest.idempotencyKey` for its `tenantId`.
