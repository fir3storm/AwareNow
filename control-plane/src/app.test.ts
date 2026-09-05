import request from 'supertest';
import { describe, expect, it } from 'vitest';

import { createApp, type ControlPlaneDependencies, type TenantPrincipal } from './app.js';
import type { SafeEngineEvent } from './security/safeEvent.js';

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

  it('rejects engine events containing a password before storage', async () => {
    const events: unknown[] = [];
    const response = await request(
      createApp(
        testDependencies({
          principalForRequest: () => ({ subjectId: 'engine_001', role: 'ENGINE', engineTenantId: 'tenant_acme' }),
          safeEventRepository: { record: async (event) => events.push(event) },
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

  it('returns aggregates for only the authenticated tenant', async () => {
    const response = await request(createApp(testDependencies())).get('/api/v1/analytics/overview');

    expect(response.status).toBe(200);
    expect(response.body).toEqual({
      sent: 100,
      opened: 60,
      clicked: 12,
      reported: 7,
      trainingCompleted: 20,
    });
  });
});

function testDependencies(overrides: Partial<ControlPlaneDependencies> = {}): ControlPlaneDependencies {
  const dependencies: ControlPlaneDependencies = {
    principalForRequest: (): TenantPrincipal => ({
      subjectId: 'user_001',
      role: 'TENANT_MEMBER',
      tenantId: 'tenant_acme',
    }),
    tenantRepository: { findById: async () => tenant },
    safeEventRepository: { record: async () => ({ inserted: true }) },
    analyticsRepository: {
      getOverview: async () => ({
        sent: 100,
        opened: 60,
        clicked: 12,
        reported: 7,
        trainingCompleted: 20,
      }),
    },
  };

  return { ...dependencies, ...overrides };
}
