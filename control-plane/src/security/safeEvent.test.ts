import { describe, expect, it } from 'vitest';

import { parseSafeEngineEvent } from './safeEvent.js';

const validEvent = {
  eventId: 'evt_001',
  tenantId: 'tenant_acme',
  campaignId: 'campaign_001',
  recipientRef: 'recipient_001',
  type: 'form_submitted',
  occurredAt: '2026-09-05T12:00:00.000Z',
  fieldNames: ['email', 'department'],
  fieldCount: 2,
};

describe('parseSafeEngineEvent', () => {
  it('accepts field metadata while preserving no submitted values', () => {
    expect(parseSafeEngineEvent(validEvent, 'tenant_acme')).toEqual(validEvent);
  });

  it.each(['password', 'credential', 'token', 'secret', 'value'])('rejects credential-like key %s', (key) => {
    expect(() => parseSafeEngineEvent({ ...validEvent, [key]: 'must-not-store' }, 'tenant_acme')).toThrow(/forbidden/i);
  });

  it('rejects an event whose tenant differs from the authenticated engine tenant', () => {
    expect(() => parseSafeEngineEvent(validEvent, 'tenant_other')).toThrow(/tenant/i);
  });

  it('rejects unknown event keys and malformed timestamps', () => {
    expect(() => parseSafeEngineEvent({ ...validEvent, extra: 'not-allowed' }, 'tenant_acme')).toThrow(/unknown/i);
    expect(() => parseSafeEngineEvent({ ...validEvent, occurredAt: 'not-a-timestamp' }, 'tenant_acme')).toThrow(
      /timestamp/i,
    );
  });

  it('rejects an unsupported event type', () => {
    expect(() => parseSafeEngineEvent({ ...validEvent, type: 'form_value_captured' }, 'tenant_acme')).toThrow(/type/i);
  });
});
