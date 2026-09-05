import { redactSecrets } from '../security/redactSecrets.js';
import type { TenantPrincipal } from '../tenancy/principal.js';
import { requireTenantScope } from '../tenancy/tenantScope.js';

export type AuditActor = {
  id: string;
  type: string;
};

export type AuditAppendInput = {
  actor: AuditActor;
  action: string;
  occurredAt: string;
  metadata: Record<string, unknown>;
};

export type AuditRecord = Readonly<{
  actor: Readonly<AuditActor>;
  tenantId: string;
  action: string;
  occurredAt: string;
  metadata: Readonly<Record<string, unknown>>;
}>;

export type AuditRepository = {
  append(record: AuditRecord): Promise<void>;
};

export type AuditService = {
  append(input: AuditAppendInput, principal: TenantPrincipal): Promise<AuditRecord>;
};

export function createAuditService(repository: AuditRepository): AuditService {
  return {
    async append(input: AuditAppendInput, principal: TenantPrincipal): Promise<AuditRecord> {
      const tenantId = requireTenantScope(principal);
      const record = deepFreeze({
        actor: { ...input.actor },
        tenantId,
        action: input.action,
        occurredAt: input.occurredAt,
        metadata: redactSecrets(input.metadata) as Record<string, unknown>,
      });

      await repository.append(record);
      return record;
    },
  };
}

function deepFreeze<T>(value: T, seen = new WeakSet<object>()): T {
  if (typeof value !== 'object' || value === null || seen.has(value)) {
    return value;
  }

  seen.add(value);
  for (const nestedValue of Object.values(value)) {
    deepFreeze(nestedValue, seen);
  }

  return Object.freeze(value);
}
