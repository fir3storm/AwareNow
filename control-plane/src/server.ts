import { createApp } from './app.js';

const app = createApp({
  principalForRequest: () => undefined,
  tenantRepository: { findById: async () => null },
  safeEventRepository: { record: async () => undefined },
  analyticsRepository: { getOverview: async () => ({ sent: 0, opened: 0, clicked: 0, reported: 0, trainingCompleted: 0 }) },
});

const port = Number(process.env.PORT ?? 3001);
app.listen(port, () => {
  console.info(`AwareNow control plane listening on port ${port}`);
});
