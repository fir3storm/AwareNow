import { describe, expect, it } from 'vitest';

import { createDevelopmentDependencies } from './serverDependencies.js';

describe('createDevelopmentDependencies', () => {
  it('uses a non-persistent safe-event repository that returns accepted metadata', async () => {
    const dependencies = createDevelopmentDependencies();
    const event = {
      eventId: 'evt_001',
      tenantId: 'tenant_acme',
      campaignId: 'campaign_001',
      recipientRef: 'recipient_001',
      type: 'link_clicked' as const,
      occurredAt: '2026-09-05T12:00:00.000Z',
    };

    await expect(dependencies.safeEventRepository.findByTenantAndExternalEventId('tenant_acme', 'evt_001')).resolves.toBe(
      null,
    );
    await expect(dependencies.safeEventRepository.record(event)).resolves.toEqual(event);
  });
});
