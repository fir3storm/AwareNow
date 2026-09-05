import express, { type Request, type Response } from 'express';

import { recordSafeEvent, type SafeEventRepository } from './events/eventService.js';
import type { TenantPrincipal } from './tenancy/principal.js';
import { requireTenantScope } from './tenancy/tenantScope.js';

export type TenantLifecycle = 'PROVISIONING' | 'ACTIVE' | 'SUSPENDED' | 'FAILED';

export type TenantView = {
  id: string;
  displayName: string;
  slug: string;
  lifecycle: TenantLifecycle;
};

export type AwarenessOverview = {
  sent: number;
  opened: number;
  clicked: number;
  reported: number;
  trainingCompleted: number;
};

export type ControlPlaneDependencies = {
  principalForRequest: (request: Request) => TenantPrincipal | undefined;
  tenantRepository: { findById: (tenantId: string) => Promise<TenantView | null> };
  safeEventRepository: SafeEventRepository;
  analyticsRepository: { getOverview: (tenantId: string) => Promise<AwarenessOverview> };
};

export function createApp(dependencies: ControlPlaneDependencies) {
  const app = express();
  app.use(express.json({ limit: '32kb' }));

  app.get('/api/v1/health', (_request, response) => {
    response.status(200).json({ status: 'ok', service: 'awarenow-control-plane' });
  });

  app.get('/api/v1/tenants/current', async (request, response) => {
    const tenantId = requireTenantId(dependencies.principalForRequest(request));
    const tenant = await dependencies.tenantRepository.findById(tenantId);
    if (tenant === null) {
      response.status(404).json({ error: 'Tenant not found.' });
      return;
    }
    response.status(200).json(tenant);
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
      response.status(200).json(await dependencies.analyticsRepository.getOverview(tenantId));
    } catch (error) {
      sendRequestError(response, error);
    }
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
