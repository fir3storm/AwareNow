import type { ControlPlaneDependencies } from './app.js';
import { createAuditService } from './audit/auditService.js';

/**
 * Local-only dependencies used before production authentication and persistence
 * are connected. They never retain recipient or submission data.
 */
export function createDevelopmentDependencies(): ControlPlaneDependencies {
  return {
    principalForRequest: () => undefined,
    tenantRepository: {
      findById: async () => null,
      create: async (input) => ({
        id: 'development-tenant',
        displayName: input.displayName,
        slug: input.slug,
        lifecycle: 'PROVISIONING',
      }),
    },
    safeEventRepository: {
      findByTenantAndExternalEventId: async () => null,
      record: async (event) => event,
    },
    analyticsRepository: {
      listSafeEvents: async () => [],
    },
    auditService: createAuditService({ append: async () => undefined }),
    provisioningDependencies: {
      tenants: {
        findById: async () => null,
        updateLifecycle: async () => undefined,
      },
      audit: {
        append: async () => undefined,
      },
      provisioner: {
        provision: async () => undefined,
      },
    },
  };
}
