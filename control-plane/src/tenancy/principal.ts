export type TenantRole = 'platform_admin' | 'tenant_member' | 'tenant_engine';

export type TenantPrincipal = {
  subjectId: string;
  role: TenantRole;
  tenantId?: string;
  engineTenantId?: string;
};
