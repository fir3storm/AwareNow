import {
  PlatformRole,
  PrismaClient,
  TenantLifecycle,
  TenantRole,
} from '@prisma/client';

const prisma = new PrismaClient();

const demoTenant = {
  id: 'b26b47b2-c5f9-422e-a4a6-39ab00e7c001',
  slug: 'awarenow-demo',
  displayName: 'AwareNow Demo',
  lifecycle: TenantLifecycle.PROVISIONING,
} as const;

const demoOperatorId = 'f28f6304-2f7e-487c-983a-2637e130c001';

async function main() {
  const tenant = await prisma.tenant.upsert({
    where: { slug: demoTenant.slug },
    update: {
      displayName: demoTenant.displayName,
      lifecycle: demoTenant.lifecycle,
    },
    create: demoTenant,
  });

  await prisma.tenantMembership.upsert({
    where: {
      tenantId_userId: {
        tenantId: tenant.id,
        userId: demoOperatorId,
      },
    },
    update: {
      tenantRole: TenantRole.OWNER,
      platformRole: PlatformRole.ADMIN,
    },
    create: {
      id: '44f0100d-0df9-4fcb-9760-8e9bf4e81001',
      tenantId: tenant.id,
      userId: demoOperatorId,
      tenantRole: TenantRole.OWNER,
      platformRole: PlatformRole.ADMIN,
    },
  });

  await prisma.engineInstance.upsert({
    where: { tenantId: tenant.id },
    update: {
      lifecycle: TenantLifecycle.PROVISIONING,
      engineBaseUrl: 'https://engine.awarenow-demo.invalid',
      databaseReference: 'postgresql://reference/awarenow-demo-engine',
      workerIdentityReference: 'identity://reference/awarenow-demo-worker',
      deliveryCredentialReference: 'secret://reference/awarenow-demo-delivery',
      controlCredentialReference: 'secret://reference/awarenow-demo-control',
      domainRoute: 'training.awarenow-demo.invalid',
    },
    create: {
      id: '75d3edc4-86f7-4375-9fe7-dc6b7c99d001',
      tenantId: tenant.id,
      lifecycle: TenantLifecycle.PROVISIONING,
      engineBaseUrl: 'https://engine.awarenow-demo.invalid',
      databaseReference: 'postgresql://reference/awarenow-demo-engine',
      workerIdentityReference: 'identity://reference/awarenow-demo-worker',
      deliveryCredentialReference: 'secret://reference/awarenow-demo-delivery',
      controlCredentialReference: 'secret://reference/awarenow-demo-control',
      domainRoute: 'training.awarenow-demo.invalid',
    },
  });
}

main()
  .then(() => prisma.$disconnect())
  .catch(async (error: unknown) => {
    await prisma.$disconnect();
    throw error;
  });
