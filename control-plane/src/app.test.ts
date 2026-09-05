import request from 'supertest';
import { describe, expect, it } from 'vitest';

import { createApp, type ControlPlaneDependencies } from './app.js';
import type { SafeEventRepository } from './events/eventService.js';
import type { SafeEngineEvent } from './security/safeEvent.js';
import type { TenantPrincipal } from './tenancy/principal.js';

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
    tenantRepository: { findById: async () => tenant },
    safeEventRepository: repositoryRecording([]),
    analyticsRepository: { listSafeEvents: async () => [] },
  };

  return { ...dependencies, ...overrides };
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
