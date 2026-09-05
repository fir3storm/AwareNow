import { createApp } from './app.js';
import { loadEnvironment } from './config/env.js';

const environment = loadEnvironment();
const app = createApp({
  principalForRequest: () => undefined,
  tenantRepository: { findById: async () => null },
  safeEventRepository: { record: async () => undefined },
  analyticsRepository: { getOverview: async () => ({ sent: 0, opened: 0, clicked: 0, reported: 0, trainingCompleted: 0 }) },
});

app.listen(environment.port, () => {
  console.info(`AwareNow control plane listening on port ${environment.port}`);
});
