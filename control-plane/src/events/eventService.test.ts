import { describe, expect, it } from 'vitest';

import { recordSafeEvent, type SafeEventRepository } from './eventService.js';
import type { SafeEngineEvent } from '../security/safeEvent.js';
import type { TenantPrincipal } from '../tenancy/principal.js';

const enginePrincipal: TenantPrincipal = {
  subjectId: 'engine_001',
  role: 'tenant_engine',
  engineTenantId: 'tenant_acme',
};

const validEvent = {
  eventId: 'evt_001',
  tenantId: 'tenant_acme',
  campaignId: 'campaign_001',
  recipientRef: 'recipient_001',
  type: 'link_clicked',
  occurredAt: '2026-09-05T12:00:00.000Z',
};

class InMemorySafeEventRepository implements SafeEventRepository {
  readonly recorded: SafeEngineEvent[] = [];

  async findByTenantAndExternalEventId(tenantId: string, eventId: string): Promise<SafeEngineEvent | null> {
    return this.recorded.find((event) => event.tenantId === tenantId && event.eventId === eventId) ?? null;
  }

  async record(event: SafeEngineEvent): Promise<SafeEngineEvent> {
    this.recorded.push(event);
    return event;
  }
}

describe('recordSafeEvent', () => {
  it('stores an event under the authenticated engine tenant scope', async () => {
    const repository = new InMemorySafeEventRepository();

    const result = await recordSafeEvent(validEvent, enginePrincipal, repository);

    expect(result.tenantId).toBe('tenant_acme');
    expect(repository.recorded).toEqual([validEvent]);
  });

  it('returns the existing event without storing a duplicate external event ID', async () => {
    const repository = new InMemorySafeEventRepository();

    await recordSafeEvent(validEvent, enginePrincipal, repository);
    const result = await recordSafeEvent(validEvent, enginePrincipal, repository);

    expect(result).toEqual(validEvent);
    expect(repository.recorded).toHaveLength(1);
  });

  it('rejects an unsafe event before it reaches the repository', async () => {
    const repository = new InMemorySafeEventRepository();

    await expect(
      recordSafeEvent({ ...validEvent, password: 'must-not-store' }, enginePrincipal, repository),
    ).rejects.toThrow(/forbidden/i);

    expect(repository.recorded).toEqual([]);
  });

  it('rejects a tenant member from storing an engine event', async () => {
    const repository = new InMemorySafeEventRepository();
    const member: TenantPrincipal = {
      subjectId: 'user_001',
      role: 'tenant_member',
      tenantId: 'tenant_acme',
    };

    await expect(recordSafeEvent(validEvent, member, repository)).rejects.toThrow(/engine tenant scope/i);

    expect(repository.recorded).toEqual([]);
  });
});
