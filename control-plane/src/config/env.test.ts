import { describe, expect, it } from 'vitest';

import { loadEnvironment } from './env.js';

describe('loadEnvironment', () => {
  it('uses the safe default port when PORT is absent', () => {
    expect(loadEnvironment({})).toMatchObject({ port: 3001 });
  });

  it('accepts a valid explicit port without requiring live service credentials', () => {
    expect(loadEnvironment({ PORT: '4100' })).toMatchObject({ port: 4100 });
  });

  it('rejects an invalid listening port before startup', () => {
    expect(() => loadEnvironment({ PORT: '0' })).toThrow();
  });

  it('rejects a blank control token when one is supplied', () => {
    expect(() => loadEnvironment({ AWARENOW_CONTROL_TOKEN: '   ' })).toThrow();
  });
});
