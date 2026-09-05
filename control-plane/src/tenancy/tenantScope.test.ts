import { describe, expect, it } from 'vitest';

import { requireTenantScope } from './tenantScope.js';
import type { TenantPrincipal } from './principal.js';

describe('requireTenantScope', () => {
  it('returns the authenticated tenant member scope', () => {
    const principal: TenantPrincipal = {
      subjectId: 'user_001',
      role: 'tenant_member',
      tenantId: 'tenant_acme',
    };

    expect(requireTenantScope(principal)).toBe('tenant_acme');
  });

  it('rejects a platform administrator without an explicitly selected tenant', () => {
    const principal: TenantPrincipal = {
      subjectId: 'admin_001',
      role: 'platform_admin',
    };

    expect(() => requireTenantScope(principal)).toThrow(/tenant scope/i);
  });

  it('uses an explicitly selected tenant scope for a platform administrator', () => {
    const principal: TenantPrincipal = {
      subjectId: 'admin_001',
      role: 'platform_admin',
      tenantId: 'tenant_acme',
    };

    expect(requireTenantScope(principal)).toBe('tenant_acme');
  });

  it('does not use a caller-supplied tenant value to override the authenticated scope', () => {
    const principal: TenantPrincipal = {
      subjectId: 'user_001',
      role: 'tenant_member',
      tenantId: 'tenant_acme',
    };
    const callerSuppliedTenantId = 'tenant_other';

    expect(requireTenantScope(principal)).toBe('tenant_acme');
    expect(requireTenantScope(principal)).not.toBe(callerSuppliedTenantId);
  });
});
