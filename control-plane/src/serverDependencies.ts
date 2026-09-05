import type { ControlPlaneDependencies } from './app.js';

/**
 * Local-only dependencies used before production authentication and persistence
 * are connected. They never retain recipient or submission data.
 */
export function createDevelopmentDependencies(): ControlPlaneDependencies {
  return {
    principalForRequest: () => undefined,
    tenantRepository: { findById: async () => null },
    safeEventRepository: {
      findByTenantAndExternalEventId: async () => null,
      record: async (event) => event,
    },
    analyticsRepository: {
      listSafeEvents: async () => [],
    },
  };
}
