import type { SafeEngineEvent } from '../security/safeEvent.js';

export type AwarenessOverview = {
  sent: number;
  opened: number;
  clicked: number;
  reported: number;
  trainingCompleted: number;
};

export type SafeEventAnalyticsRepository = {
  listSafeEvents(tenantId: string): Promise<SafeEngineEvent[]>;
};

export async function getTenantAwarenessOverview(
  tenantId: string,
  repository: SafeEventAnalyticsRepository,
): Promise<AwarenessOverview> {
  const events = await repository.listSafeEvents(tenantId);
  return events.filter((event) => event.tenantId === tenantId).reduce(addEventToOverview, emptyOverview());
}

function addEventToOverview(overview: AwarenessOverview, event: SafeEngineEvent): AwarenessOverview {
  switch (event.type) {
    case 'email_sent':
      return { ...overview, sent: overview.sent + 1 };
    case 'email_opened':
      return { ...overview, opened: overview.opened + 1 };
    case 'link_clicked':
      return { ...overview, clicked: overview.clicked + 1 };
    case 'report_submitted':
      return { ...overview, reported: overview.reported + 1 };
    case 'training_completed':
      return { ...overview, trainingCompleted: overview.trainingCompleted + 1 };
    default:
      return overview;
  }
}

function emptyOverview(): AwarenessOverview {
  return { sent: 0, opened: 0, clicked: 0, reported: 0, trainingCompleted: 0 };
}
