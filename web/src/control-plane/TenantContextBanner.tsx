import type { TenantLifecycle, TenantView } from './types';
import './controlPlane.css';

const lifecycleLabels: Record<TenantLifecycle, string> = {
  PROVISIONING: 'Provisioning',
  ACTIVE: 'Active',
  SUSPENDED: 'Suspended',
  FAILED: 'Needs attention',
};

interface TenantContextBannerProps {
  tenant: TenantView;
}

export function TenantContextBanner({ tenant }: TenantContextBannerProps) {
  return (
    <section className="control-plane-tenant-banner" aria-label="Current tenant">
      <div>
        <p className="control-plane-eyebrow">Current workspace</p>
        <h2 className="control-plane-tenant-name">{tenant.displayName}</h2>
        <p className="control-plane-tenant-slug">{tenant.slug}</p>
      </div>
      <span className={`control-plane-lifecycle control-plane-lifecycle--${tenant.lifecycle.toLowerCase()}`}>
        {lifecycleLabels[tenant.lifecycle]}
      </span>
    </section>
  );
}
