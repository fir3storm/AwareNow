import type { TenantLifecycle, TenantView } from './types';
import './controlPlane.css';

const lifecycleCopy: Record<TenantLifecycle, { title: string; detail: string }> = {
  PROVISIONING: {
    title: 'Workspace setup is in progress',
    detail: 'Your isolated awareness environment is being prepared. Campaign controls will appear when setup is complete.',
  },
  ACTIVE: {
    title: 'Workspace is ready',
    detail: 'Your isolated awareness environment is active and ready for approved program activity.',
  },
  SUSPENDED: {
    title: 'Workspace is paused',
    detail: 'Program activity is paused for this tenant. Contact a platform administrator for next steps.',
  },
  FAILED: {
    title: 'Workspace setup needs attention',
    detail: 'The environment could not be prepared. A platform administrator can review the provisioning status.',
  },
};

interface ProvisioningStatusProps {
  tenant: TenantView;
}

export function ProvisioningStatus({ tenant }: ProvisioningStatusProps) {
  const status = lifecycleCopy[tenant.lifecycle];

  return (
    <aside className="control-plane-provisioning-status" aria-live="polite">
      <p className="control-plane-eyebrow">Tenant lifecycle</p>
      <h2>{status.title}</h2>
      <p>{status.detail}</p>
    </aside>
  );
}
