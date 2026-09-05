import type { TenantPrincipal } from './principal.js';

export function requireTenantScope(principal: TenantPrincipal): string {
  if (principal.role === 'tenant_engine') {
    throw new Error('A tenant member scope is required.');
  }

  if (principal.tenantId === undefined || principal.tenantId.length === 0) {
    throw new Error('An explicit tenant scope is required.');
  }

  return principal.tenantId;
}
