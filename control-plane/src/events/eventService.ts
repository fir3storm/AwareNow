import { parseSafeEngineEvent, type SafeEngineEvent } from '../security/safeEvent.js';
import type { TenantPrincipal } from '../tenancy/principal.js';

export type SafeEventRepository = {
  findByTenantAndExternalEventId(tenantId: string, eventId: string): Promise<SafeEngineEvent | null>;
  record(event: SafeEngineEvent): Promise<SafeEngineEvent>;
};

export async function recordSafeEvent(
  input: unknown,
  principal: TenantPrincipal,
  repository: SafeEventRepository,
): Promise<SafeEngineEvent> {
  const authenticatedEngineTenantId = requireEngineTenantScope(principal);
  const event = parseSafeEngineEvent(input, authenticatedEngineTenantId);
  const existing = await repository.findByTenantAndExternalEventId(authenticatedEngineTenantId, event.eventId);

  if (existing !== null) {
    return existing;
  }

  return repository.record(event);
}

function requireEngineTenantScope(principal: TenantPrincipal): string {
  if (principal.role !== 'tenant_engine' || principal.engineTenantId === undefined || principal.engineTenantId.length === 0) {
    throw new Error('An authenticated engine tenant scope is required.');
  }

  return principal.engineTenantId;
}
