import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, expect, test } from 'vitest';
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

afterEach(cleanup);

test('renders the tenant display name, slug, and lifecycle', () => {
  render(<TenantContextBanner tenant={provisioningTenant} />);

  expect(screen.getByRole('heading', { name: 'AwareNow Demo' })).toBeInTheDocument();
  expect(screen.getByText('awarenow-demo')).toBeInTheDocument();
  expect(screen.getByText('Provisioning')).toBeInTheDocument();
});
