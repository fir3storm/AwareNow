import { describe, expect, it } from 'vitest';

import {
  provisionTenant,
  type ProvisioningDependencies,
  type ProvisioningTenant,
} from './provisionTenant.js';

function createDependencies(
  tenant: ProvisioningTenant | null,
  provision: (tenant: ProvisioningTenant) => Promise<void>,
): { dependencies: ProvisioningDependencies; updates: Array<{ tenantId: string; lifecycle: 'ACTIVE' | 'FAILED' }>; audits: Array<{ tenantId: string; action: string }> } {
  const updates: Array<{ tenantId: string; lifecycle: 'ACTIVE' | 'FAILED' }> = [];
  const audits: Array<{ tenantId: string; action: string }> = [];

  return {
    dependencies: {
      tenants: {
        findById: async () => tenant,
        updateLifecycle: async (tenantId, lifecycle) => {
          updates.push({ tenantId, lifecycle });
        },
      },
      audit: {
        append: async (entry) => {
          audits.push({ tenantId: entry.tenantId, action: entry.action });
        },
      },
      provisioner: { provision },
    },
    updates,
    audits,
  };
}

describe('provisionTenant', () => {
  it('does nothing when the tenant is already active', async () => {
    const { dependencies, updates, audits } = createDependencies(
      { id: 'tenant-active', lifecycle: 'ACTIVE' },
      async () => {
        throw new Error('must not provision an active tenant');
      },
    );

    await expect(provisionTenant('tenant-active', dependencies)).resolves.toEqual({
      tenantId: 'tenant-active',
      lifecycle: 'ACTIVE',
      changed: false,
    });
    expect(updates).toEqual([]);
    expect(audits).toEqual([]);
  });

  it('activates a provisioning tenant and appends an immutable audit entry', async () => {
    const { dependencies, updates, audits } = createDependencies(
      { id: 'tenant-ready', lifecycle: 'PROVISIONING' },
      async () => undefined,
    );

    await expect(provisionTenant('tenant-ready', dependencies)).resolves.toEqual({
      tenantId: 'tenant-ready',
      lifecycle: 'ACTIVE',
      changed: true,
    });
    expect(updates).toEqual([{ tenantId: 'tenant-ready', lifecycle: 'ACTIVE' }]);
    expect(audits).toEqual([{ tenantId: 'tenant-ready', action: 'tenant.provisioned' }]);
  });

  it('marks a provisioning tenant as failed and appends an immutable audit entry when provisioning fails', async () => {
    const { dependencies, updates, audits } = createDependencies(
      { id: 'tenant-failed', lifecycle: 'PROVISIONING' },
      async () => {
        throw new Error('isolated engine unavailable');
      },
    );

    await expect(provisionTenant('tenant-failed', dependencies)).resolves.toEqual({
      tenantId: 'tenant-failed',
      lifecycle: 'FAILED',
      changed: true,
    });
    expect(updates).toEqual([{ tenantId: 'tenant-failed', lifecycle: 'FAILED' }]);
    expect(audits).toEqual([{ tenantId: 'tenant-failed', action: 'tenant.provisioning_failed' }]);
  });

  it('returns a not-found result without provisioning, updating, or auditing', async () => {
    const { dependencies, updates, audits } = createDependencies(
      null,
      async () => {
        throw new Error('must not provision an unknown tenant');
      },
    );

    await expect(provisionTenant('tenant-missing', dependencies)).resolves.toEqual({
      tenantId: 'tenant-missing',
      lifecycle: null,
      changed: false,
      notFound: true,
    });
    expect(updates).toEqual([]);
    expect(audits).toEqual([]);
  });
});
