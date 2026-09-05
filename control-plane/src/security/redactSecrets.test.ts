import { describe, expect, it } from 'vitest';

import { redactSecrets } from './redactSecrets.js';

describe('redactSecrets', () => {
  it('redacts secret-like keys recursively while preserving ordinary context', () => {
    expect(
      redactSecrets({
        status: 'failed',
        token: 'engine-token',
        nested: { authorization: 'Bearer secret', label: 'engine' },
        entries: [{ apiKey: 'api-key', detail: 'safe' }],
      }),
    ).toEqual({
      status: 'failed',
      token: '[REDACTED]',
      nested: { authorization: '[REDACTED]', label: 'engine' },
      entries: [{ apiKey: '[REDACTED]', detail: 'safe' }],
    });
  });

  it('redacts credential-like keys without regard to casing', () => {
    expect(redactSecrets({ TOKEN: 'engine-token', ApiKey: 'provider-key' })).toEqual({
      TOKEN: '[REDACTED]',
      ApiKey: '[REDACTED]',
    });
  });
});
