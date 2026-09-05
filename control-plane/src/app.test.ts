import request from 'supertest';
import { describe, expect, it } from 'vitest';

import { createApp, type ControlPlaneDependencies, type TenantView } from './app.js';
import { createAuditService } from './audit/auditService.js';
import type { SafeEventRepository } from './events/eventService.js';
import type { ProvisioningDependencies } from './provisioning/provisionTenant.js';
import type { SafeEngineEvent } from './security/safeEvent.js';
import type { TenantPrincipal } from './tenancy/principal.js';

const platformAdmin: TenantPrincipal = {
  subjectId: 'platform_001',
  role: 'platform_admin',
};

const tenant = {
  id: 'tenant_acme',
  displayName: 'Acme Awareness',
  slug: 'acme',
  lifecycle: 'ACTIVE' as const,
};

describe('control-plane app', () => {
  it('reports a dependency-free health response', async () => {
    const response = await request(createApp(testDependencies())).get('/api/v1/health');

    expect(response.status).toBe(200);
    expect(response.body).toEqual({ status: 'ok', service: 'awarenow-control-plane' });
  });

  it('returns tenant context from the authenticated principal', async () => {
    const response = await request(createApp(testDependencies())).get('/api/v1/tenants/current');

    expect(response.status).toBe(200);
    expect(response.body).toEqual(tenant);
  });

  it('returns a generic JSON error instead of a stack trace when the tenant repository fails', async () => {
    const response = await request(
      createApp(
        testDependencies({
          tenantRepository: {
            findById: async () => {
              throw new Error('lookup failed: token=super-secret-value');
            },
            create: async (input) => ({ id: 'tenant_new', displayName: input.displayName, slug: input.slug, lifecycle: 'PROVISIONING' }),
          },
        }),
      ),
    ).get('/api/v1/tenants/current');

    expect(response.status).toBe(400);
    expect(response.type).toMatch(/json/);
    expect(response.body).toEqual({ error: 'Invalid request.' });
    expect(response.body).not.toHaveProperty('stack');
    expect(response.text).not.toContain('super-secret-value');
    expect(response.text).not.toMatch(/at\s+\S+\s+\(/);
  });

  it('returns a generic JSON error instead of a stack trace for a malformed request body', async () => {
    const response = await request(createApp(testDependencies()))
      .post('/api/v1/events/engine')
      .set('Content-Type', 'application/json')
      .send('{not valid json');

    expect(response.status).toBe(500);
    expect(response.type).toMatch(/json/);
    expect(response.body).toEqual({ error: 'Internal server error.' });
    expect(response.text).not.toMatch(/at\s+\S+\s+\(/);
  });

  it('creates a provisioning tenant and an audit entry for a platform admin', async () => {
    const response = await request(
      createApp(
        testDependencies({
          principalForRequest: () => platformAdmin,
        }),
      ),
    )
      .post('/api/v1/tenants')
      .send({ displayName: 'Globex Awareness', slug: 'globex' });

    expect(response.status).toBe(201);
    expect(response.body).toEqual({
      id: 'tenant_new',
      displayName: 'Globex Awareness',
      slug: 'globex',
      lifecycle: 'PROVISIONING',
    });
  });

  it('rejects tenant creation from a non-platform-admin principal', async () => {
    const response = await request(createApp(testDependencies())).post('/api/v1/tenants').send({
      displayName: 'Globex Awareness',
      slug: 'globex',
    });

    expect(response.status).toBe(403);
    expect(response.body).toEqual({ error: 'A platform admin scope is required.' });
  });

  it('requests idempotent provisioning for a platform admin', async () => {
    const response = await request(
      createApp(
        testDependencies({
          principalForRequest: () => platformAdmin,
        }),
      ),
    ).post('/api/v1/tenants/tenant_acme/provision');

    expect(response.status).toBe(200);
    expect(response.body).toEqual({ tenantId: 'tenant_acme', lifecycle: 'ACTIVE', changed: true });
  });

  it('rejects provisioning requests from a non-platform-admin principal', async () => {
    const response = await request(createApp(testDependencies())).post('/api/v1/tenants/tenant_acme/provision');

    expect(response.status).toBe(403);
    expect(response.body).toEqual({ error: 'A platform admin scope is required.' });
  });

  it('rejects engine events containing a password before storage', async () => {
    const events: SafeEngineEvent[] = [];
    const response = await request(
      createApp(
        testDependencies({
          principalForRequest: () => ({ subjectId: 'engine_001', role: 'tenant_engine', engineTenantId: 'tenant_acme' }),
          safeEventRepository: repositoryRecording(events),
        }),
      ),
    )
      .post('/api/v1/events/engine')
      .send({
        eventId: 'evt_001',
        tenantId: 'tenant_acme',
        campaignId: 'campaign_001',
        recipientRef: 'recipient_001',
        type: 'form_submitted',
        occurredAt: '2026-09-05T12:00:00.000Z',
        password: 'must-not-store',
      });

    expect(response.status).toBe(400);
    expect(events).toEqual([]);
  });

  it('records a valid engine event through the tenant-scoped event service', async () => {
    const events: SafeEngineEvent[] = [];
    const response = await request(
      createApp(
        testDependencies({
          principalForRequest: () => ({ subjectId: 'engine_001', role: 'tenant_engine', engineTenantId: 'tenant_acme' }),
          safeEventRepository: repositoryRecording(events),
        }),
      ),
    )
      .post('/api/v1/events/engine')
      .send({
        eventId: 'evt_002',
        tenantId: 'tenant_acme',
        campaignId: 'campaign_001',
        recipientRef: 'recipient_001',
        type: 'link_clicked',
        occurredAt: '2026-09-05T12:00:00.000Z',
      });

    expect(response.status).toBe(202);
    expect(response.body).toEqual({ accepted: true, eventId: 'evt_002' });
    expect(events).toEqual([
      {
        eventId: 'evt_002',
        tenantId: 'tenant_acme',
        campaignId: 'campaign_001',
        recipientRef: 'recipient_001',
        type: 'link_clicked',
        occurredAt: '2026-09-05T12:00:00.000Z',
      },
    ]);
  });

  it('returns aggregates for only the authenticated tenant', async () => {
    const response = await request(
      createApp(
        testDependencies({
          analyticsRepository: {
            listSafeEvents: async () => [
              analyticsEvent('email_sent'),
              analyticsEvent('email_opened'),
              analyticsEvent('link_clicked'),
              analyticsEvent('report_submitted'),
              analyticsEvent('training_completed'),
              { ...analyticsEvent('link_clicked'), tenantId: 'tenant_other' },
            ],
          },
        }),
      ),
    ).get('/api/v1/analytics/overview');

    expect(response.status).toBe(200);
    expect(response.body).toEqual({
      sent: 1,
      opened: 1,
      clicked: 1,
      reported: 1,
      trainingCompleted: 1,
    });
  });
});

function testDependencies(overrides: Partial<ControlPlaneDependencies> = {}): ControlPlaneDependencies {
  const dependencies: ControlPlaneDependencies = {
    principalForRequest: (): TenantPrincipal => ({
      subjectId: 'user_001',
      role: 'tenant_member',
      tenantId: 'tenant_acme',
    }),
    tenantRepository: {
      findById: async () => tenant,
      create: async (input): Promise<TenantView> => ({
        id: 'tenant_new',
        displayName: input.displayName,
        slug: input.slug,
        lifecycle: 'PROVISIONING',
      }),
    },
    safeEventRepository: repositoryRecording([]),
    analyticsRepository: { listSafeEvents: async () => [] },
    auditService: createAuditService({ append: async () => undefined }),
    provisioningDependencies: testProvisioningDependencies(),
  };

  return { ...dependencies, ...overrides };
}

function testProvisioningDependencies(): ProvisioningDependencies {
  return {
    tenants: {
      findById: async () => ({ id: 'tenant_acme', lifecycle: 'PROVISIONING' }),
      updateLifecycle: async () => undefined,
    },
    audit: {
      append: async () => undefined,
    },
    provisioner: {
      provision: async () => undefined,
    },
  };
}

function repositoryRecording(events: SafeEngineEvent[]): SafeEventRepository {
  return {
    findByTenantAndExternalEventId: async (tenantId, eventId) => (
      events.find((event) => event.tenantId === tenantId && event.eventId === eventId) ?? null
    ),
    record: async (event) => {
      events.push(event);
      return event;
    },
  };
}

function analyticsEvent(type: SafeEngineEvent['type']): SafeEngineEvent {
  return {
    eventId: `analytics_${type}`,
    tenantId: 'tenant_acme',
    campaignId: 'campaign_001',
    recipientRef: 'recipient_001',
    type,
    occurredAt: '2026-09-05T12:00:00.000Z',
  };
}
