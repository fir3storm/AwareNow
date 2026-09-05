import { describe, expect, it } from 'vitest';

import { createAuditService, type AuditRepository } from './auditService.js';
import type { TenantPrincipal } from '../tenancy/principal.js';

const tenantMember: TenantPrincipal = {
  subjectId: 'user_001',
  role: 'tenant_member',
  tenantId: 'tenant_acme',
};

class RecordingAuditRepository implements AuditRepository {
  readonly records: unknown[] = [];

  async append(record: Parameters<AuditRepository['append']>[0]): Promise<void> {
    this.records.push(record);
  }
}

describe('createAuditService', () => {
  it('redacts credential-like metadata recursively without mutating the caller input', async () => {
    const repository = new RecordingAuditRepository();
    const service = createAuditService(repository);
    const metadata = {
      status: 'updated',
      nested: { authorization: 'Bearer engine-token', label: 'engine' },
      attempts: [{ apiKey: 'provider-key', result: 'rejected' }],
    };

    await service.append(
      {
        actor: { id: 'user_001', type: 'tenant_member' },
        action: 'engine.settings.updated',
        occurredAt: '2026-09-05T14:00:00.000Z',
        metadata,
      },
      tenantMember,
    );

    expect(repository.records).toEqual([
      {
        actor: { id: 'user_001', type: 'tenant_member' },
        tenantId: 'tenant_acme',
        action: 'engine.settings.updated',
        occurredAt: '2026-09-05T14:00:00.000Z',
        metadata: {
          status: 'updated',
          nested: { authorization: '[REDACTED]', label: 'engine' },
          attempts: [{ apiKey: '[REDACTED]', result: 'rejected' }],
        },
      },
    ]);
    expect(metadata.nested.authorization).toBe('Bearer engine-token');
    expect(metadata.attempts[0]?.apiKey).toBe('provider-key');
  });

  it('rejects an append when the caller has no tenant scope', async () => {
    const repository = new RecordingAuditRepository();
    const service = createAuditService(repository);
    const unscopedPrincipal: TenantPrincipal = {
      subjectId: 'platform_001',
      role: 'platform_admin',
    };

    await expect(
      service.append(
        {
          actor: { id: 'platform_001', type: 'platform_admin' },
          action: 'tenant.created',
          occurredAt: '2026-09-05T14:00:00.000Z',
          metadata: {},
        },
        unscopedPrincipal,
      ),
    ).rejects.toThrow(/tenant scope/i);

    expect(repository.records).toEqual([]);
  });

  it('hands the repository a deeply immutable audit record', async () => {
    const repository = new RecordingAuditRepository();
    const service = createAuditService(repository);

    await service.append(
      {
        actor: { id: 'user_001', type: 'tenant_member' },
        action: 'campaign.created',
        occurredAt: '2026-09-05T14:00:00.000Z',
        metadata: { campaign: { id: 'campaign_001' } },
      },
      tenantMember,
    );

    const [record] = repository.records;
    expect(Object.isFrozen(record)).toBe(true);
    expect(Object.isFrozen((record as { actor: unknown }).actor)).toBe(true);
    expect(Object.isFrozen((record as { metadata: { campaign: unknown } }).metadata)).toBe(true);
    expect(Object.isFrozen((record as { metadata: { campaign: unknown } }).metadata.campaign)).toBe(true);
  });
});
