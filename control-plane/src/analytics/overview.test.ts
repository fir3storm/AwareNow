import { describe, expect, it } from 'vitest';

import { getTenantAwarenessOverview } from './overview.js';

describe('getTenantAwarenessOverview', () => {
  it('maps safe event types to aggregate awareness totals', async () => {
    const overview = await getTenantAwarenessOverview('tenant_acme', {
      listSafeEvents: async () => [
        event('email_sent'),
        event('email_opened'),
        event('link_clicked'),
        event('report_submitted'),
        event('training_completed'),
        event('landing_page_viewed'),
      ],
    });

    expect(overview).toEqual({ sent: 1, opened: 1, clicked: 1, reported: 1, trainingCompleted: 1 });
  });

  it('uses the requested tenant and ignores a malformed cross-tenant repository record', async () => {
    const requestedTenantIds: string[] = [];
    const overview = await getTenantAwarenessOverview('tenant_acme', {
      listSafeEvents: async (tenantId) => {
        requestedTenantIds.push(tenantId);
        return [event('email_sent'), { ...event('link_clicked'), tenantId: 'tenant_other' }];
      },
    });

    expect(requestedTenantIds).toEqual(['tenant_acme']);
    expect(overview).toEqual({ sent: 1, opened: 0, clicked: 0, reported: 0, trainingCompleted: 0 });
  });
});

function event(type: 'email_sent' | 'email_opened' | 'link_clicked' | 'report_submitted' | 'training_completed' | 'landing_page_viewed') {
  return {
    eventId: `event_${type}`,
    tenantId: 'tenant_acme',
    campaignId: 'campaign_001',
    recipientRef: 'recipient_001',
    type,
    occurredAt: '2026-09-05T12:00:00.000Z',
  };
}
