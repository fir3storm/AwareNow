/// <reference types="node" />

import assert from 'node:assert/strict';
import test from 'node:test';
import { renderToStaticMarkup } from 'react-dom/server';
import { TenantContextBanner } from './TenantContextBanner';
import type { TenantView } from './types';

const provisioningTenant: TenantView = {
  id: 'tenant-demo',
  displayName: 'AwareNow Demo',
  slug: 'awarenow-demo',
  lifecycle: 'PROVISIONING',
};

// @ts-expect-error Tenant presentation data must never accept control secrets.
const unsafeTenant: TenantView = { ...provisioningTenant, engineToken: 'not-allowed' };
void unsafeTenant;

test('renders the tenant display name, slug, and lifecycle', () => {
  const markup = renderToStaticMarkup(<TenantContextBanner tenant={provisioningTenant} />);

  assert.match(markup, /AwareNow Demo/);
  assert.match(markup, /awarenow-demo/);
  assert.match(markup, /Provisioning/);
});
