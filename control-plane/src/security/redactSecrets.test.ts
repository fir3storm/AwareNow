import { describe, expect, it } from 'vitest';

import { redactSecrets } from './redactSecrets.js';

describe('redactSecrets', () => {
  it('redacts nested credential-like values while retaining ordinary values', () => {
    expect(
      redactSecrets({
        requestId: 'request-123',
        configuration: {
          token: 'engine-token',
          apiKey: 'provider-key',
          secret: 'shared-secret',
          authorization: 'Bearer engine-token',
          retryCount: 3,
        },
      }),
    ).toEqual({
      requestId: 'request-123',
      configuration: {
        token: '[REDACTED]',
        apiKey: '[REDACTED]',
        secret: '[REDACTED]',
        authorization: '[REDACTED]',
        retryCount: 3,
      },
    });
  });

  it('redacts credential-like keys without regard to casing', () => {
    expect(redactSecrets({ TOKEN: 'engine-token', ApiKey: 'provider-key' })).toEqual({
      TOKEN: '[REDACTED]',
      ApiKey: '[REDACTED]',
    });
  });
});
