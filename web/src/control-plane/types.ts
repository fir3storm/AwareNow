export type TenantLifecycle = 'PROVISIONING' | 'ACTIVE' | 'SUSPENDED' | 'FAILED';

export interface TenantView {
  id: string;
  displayName: string;
  slug: string;
  lifecycle: TenantLifecycle;
}

export interface AwarenessMetrics {
  sent: number;
  opened: number;
  clicked: number;
  reported: number;
  trainingCompleted: number;
}
