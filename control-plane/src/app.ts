import express, { type NextFunction, type Request, type Response } from 'express';

import { getTenantAwarenessOverview, type SafeEventAnalyticsRepository } from './analytics/overview.js';
import type { AuditService } from './audit/auditService.js';
import { recordSafeEvent, type SafeEventRepository } from './events/eventService.js';
import { provisionTenant, type ProvisioningDependencies } from './provisioning/provisionTenant.js';
import { redactSecrets } from './security/redactSecrets.js';
import type { TenantPrincipal } from './tenancy/principal.js';
import { requireTenantScope } from './tenancy/tenantScope.js';

export type TenantLifecycle = 'PROVISIONING' | 'ACTIVE' | 'SUSPENDED' | 'FAILED';

export type TenantView = {
  id: string;
  displayName: string;
  slug: string;
  lifecycle: TenantLifecycle;
};

export type TenantCreateInput = {
  displayName: string;
  slug: string;
};

export type ControlPlaneDependencies = {
  principalForRequest: (request: Request) => TenantPrincipal | undefined;
  tenantRepository: {
    findById: (tenantId: string) => Promise<TenantView | null>;
    create: (input: TenantCreateInput) => Promise<TenantView>;
  };
  safeEventRepository: SafeEventRepository;
  analyticsRepository: SafeEventAnalyticsRepository;
  auditService: AuditService;
  provisioningDependencies: ProvisioningDependencies;
};

export function createApp(dependencies: ControlPlaneDependencies) {
  const app = express();
  app.use(express.json({ limit: '32kb' }));

  app.get('/api/v1/health', (_request, response) => {
    response.status(200).json({ status: 'ok', service: 'awarenow-control-plane' });
  });

  app.get('/api/v1/tenants/current', async (request, response) => {
    try {
      const tenantId = requireTenantId(dependencies.principalForRequest(request));
      const tenant = await dependencies.tenantRepository.findById(tenantId);
      if (tenant === null) {
        response.status(404).json({ error: 'Tenant not found.' });
        return;
      }
      response.status(200).json(tenant);
    } catch (error) {
      sendRequestError(response, error);
    }
  });

  app.post('/api/v1/tenants', async (request, response) => {
    try {
      const principal = requirePlatformAdmin(dependencies.principalForRequest(request));
      const input = parseTenantCreateInput(request.body);
      const tenant = await dependencies.tenantRepository.create(input);
      await dependencies.auditService.append(
        {
          actor: { id: principal.subjectId, type: principal.role },
          action: 'tenant.created',
          occurredAt: new Date().toISOString(),
          metadata: { tenantId: tenant.id, slug: tenant.slug },
        },
        { ...principal, tenantId: tenant.id },
      );
      response.status(201).json(tenant);
    } catch (error) {
      sendRequestError(response, error);
    }
  });

  app.post('/api/v1/tenants/:tenantId/provision', async (request, response) => {
    try {
      requirePlatformAdmin(dependencies.principalForRequest(request));
      const result = await provisionTenant(request.params.tenantId, dependencies.provisioningDependencies);
      if (result.notFound === true) {
        response.status(404).json({ error: 'Tenant not found.' });
        return;
      }
      response.status(200).json({ tenantId: result.tenantId, lifecycle: result.lifecycle, changed: result.changed });
    } catch (error) {
      sendRequestError(response, error);
    }
  });

  app.post('/api/v1/events/engine', async (request, response) => {
    try {
      const principal = requireEnginePrincipal(dependencies.principalForRequest(request));
      const event = await recordSafeEvent(request.body, principal, dependencies.safeEventRepository);
      response.status(202).json({ accepted: true, eventId: event.eventId });
    } catch (error) {
      sendRequestError(response, error);
    }
  });

  app.get('/api/v1/analytics/overview', async (request, response) => {
    try {
      const tenantId = requireTenantId(dependencies.principalForRequest(request));
      response.status(200).json(await getTenantAwarenessOverview(tenantId, dependencies.analyticsRepository));
    } catch (error) {
      sendRequestError(response, error);
    }
  });

  app.use((err: unknown, _request: Request, response: Response, _next: NextFunction) => {
    const safeError =
      err instanceof Error
        ? (redactSecrets({ message: err.message, name: err.name }) as { message: string; name: string })
        : redactSecrets({ message: 'Unknown error.', name: 'UnknownError' });
    console.error('Unhandled control-plane request error', safeError);
    response.status(500).json({ error: 'Internal server error.' });
  });

  return app;
}

function requireTenantId(principal: TenantPrincipal | undefined): string {
  if (principal === undefined) {
    throw new RequestScopeError(401, 'Authentication is required.');
  }
  try {
    return requireTenantScope(principal);
  } catch {
    throw new RequestScopeError(403, 'A tenant scope is required.');
  }
}

function requirePlatformAdmin(principal: TenantPrincipal | undefined): TenantPrincipal {
  if (principal === undefined) {
    throw new RequestScopeError(401, 'Authentication is required.');
  }
  if (principal.role !== 'platform_admin') {
    throw new RequestScopeError(403, 'A platform admin scope is required.');
  }
  return principal;
}

function parseTenantCreateInput(body: unknown): TenantCreateInput {
  if (typeof body !== 'object' || body === null) {
    throw new Error('Invalid request.');
  }
  const { displayName, slug } = body as Record<string, unknown>;
  if (typeof displayName !== 'string' || displayName.trim().length === 0) {
    throw new Error('Invalid request.');
  }
  if (typeof slug !== 'string' || slug.trim().length === 0) {
    throw new Error('Invalid request.');
  }
  return { displayName, slug };
}

function requireEnginePrincipal(principal: TenantPrincipal | undefined): TenantPrincipal {
  if (principal === undefined || principal.role !== 'tenant_engine') {
    throw new RequestScopeError(401, 'Engine authentication is required.');
  }
  if (principal.engineTenantId === undefined || principal.engineTenantId.length === 0) {
    throw new RequestScopeError(403, 'An engine tenant scope is required.');
  }
  return principal;
}

function sendRequestError(response: Response, error: unknown) {
  if (error instanceof RequestScopeError) {
    response.status(error.status).json({ error: error.message });
    return;
  }
  response.status(400).json({ error: 'Invalid request.' });
}

class RequestScopeError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}
