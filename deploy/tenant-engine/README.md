# Isolated tenant engine reference topology

This directory is a reference-only topology for an authorized AwareNow
security-awareness tenant. It is deliberately not a deployment guide and does
not contain working credentials.

## Required isolation invariants

- Use one Compose project for exactly one tenant. Set a unique
  `AWARENOW_TENANT_SLUG`; Compose names the project and network from it.
- The tenant network is named `awarenow-${AWARENOW_TENANT_SLUG}` and is
  internal. Do not attach another tenant's engine or database to it.
- Every tenant gets its own `engine` service and `engine-database` volume. The
  `ENGINE_DATABASE_URL` must resolve to that tenant's database only.
- Every tenant has its own worker identity. Keep that identity outside this
  repository and associate it only with that tenant's provisioned engine.
- `DELIVERY_CREDENTIAL_REF` is a reference to a tenant-specific delivery
  credential. It is not an SMTP password, API key, or OAuth token. Never share
  the reference or credential between tenants.
- `CAMPAIGN_DOMAIN` is a tenant-specific campaign and tracking domain. It must
  not route to another tenant.
- `AWARENOW_CONTROL_TOKEN`, `ENGINE_DATABASE_URL`, and `DATABASE_PASSWORD_REF`
  are secret references in this template. Resolve them at runtime through an
  approved secret manager; do not commit a populated `.env` file.

## Files

- `docker-compose.yml` defines an engine and its separate PostgreSQL database.
  It does not publish the engine port to the host.
- `.env.example` contains placeholders only. Copy it only into an approved
  secret-injection workflow, not into version control.
- `nginx.conf.template` is public campaign-domain ingress. It returns `404` for
  `/api/v1/control/` so the private engine control API cannot be reached through
  public Nginx. Attach any internal control-plane proxy separately to the
  tenant's private network.

## Safe validation

This topology is intentionally non-deploying. After supplying only non-secret
test references in a local environment, an operator may render it without
starting services:

```powershell
docker compose -f deploy/tenant-engine/docker-compose.yml config
```

Do not run `docker compose up` from this reference directory. Provisioning,
domain routing, credential resolution, and worker-identity issuance belong to
the control-plane workflow and require tenant authorization.
