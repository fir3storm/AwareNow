import { createApp } from './app.js';
import { loadEnvironment } from './config/env.js';
import { createDevelopmentDependencies } from './serverDependencies.js';

const environment = loadEnvironment();
const app = createApp(createDevelopmentDependencies());

app.listen(environment.port, () => {
  console.info(`AwareNow control plane listening on port ${environment.port}`);
});
