export type TenantLifecycle = 'PROVISIONING' | 'ACTIVE' | 'SUSPENDED' | 'FAILED';

export type ProvisioningTenant = {
  id: string;
  lifecycle: TenantLifecycle;
};

export type TenantProvisioningRepository = {
  findById(tenantId: string): Promise<ProvisioningTenant | null>;
  updateLifecycle(tenantId: string, lifecycle: 'ACTIVE' | 'FAILED'): Promise<void>;
};

export type ImmutableAuditEntry = {
  tenantId: string;
  action: 'tenant.provisioned' | 'tenant.provisioning_failed';
};

export type ImmutableAuditRepository = {
  append(entry: ImmutableAuditEntry): Promise<void>;
};

export type TenantProvisioner = {
  provision(tenant: ProvisioningTenant): Promise<void>;
};

export type ProvisioningDependencies = {
  tenants: TenantProvisioningRepository;
  audit: ImmutableAuditRepository;
  provisioner: TenantProvisioner;
};

export type ProvisioningResult = {
  tenantId: string;
  lifecycle: TenantLifecycle | null;
  changed: boolean;
  notFound?: true;
};

export async function provisionTenant(
  tenantId: string,
  dependencies: ProvisioningDependencies,
): Promise<ProvisioningResult> {
  const tenant = await dependencies.tenants.findById(tenantId);

  if (tenant === null) {
    return { tenantId, lifecycle: null, changed: false, notFound: true };
  }

  if (tenant.lifecycle !== 'PROVISIONING') {
    return { tenantId: tenant.id, lifecycle: tenant.lifecycle, changed: false };
  }

  try {
    await dependencies.provisioner.provision(tenant);
    await dependencies.tenants.updateLifecycle(tenant.id, 'ACTIVE');
    await dependencies.audit.append({ tenantId: tenant.id, action: 'tenant.provisioned' });
    return { tenantId: tenant.id, lifecycle: 'ACTIVE', changed: true };
  } catch {
    await dependencies.tenants.updateLifecycle(tenant.id, 'FAILED');
    await dependencies.audit.append({ tenantId: tenant.id, action: 'tenant.provisioning_failed' });
    return { tenantId: tenant.id, lifecycle: 'FAILED', changed: true };
  }
}
