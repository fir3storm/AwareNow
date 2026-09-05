import { describe, expect, it } from 'vitest';

import { parseSafeEngineEvent } from './safeEvent.js';

const validEvent = {
  eventId: 'event-123',
  tenantId: 'tenant-123',
  campaignId: 'campaign-123',
  recipientRef: 'recipient-123',
  type: 'link_clicked',
  occurredAt: '2026-09-05T12:30:00.000Z',
  fieldNames: ['department', 'role'],
  fieldCount: 2,
};

describe('parseSafeEngineEvent', () => {
  it('returns a safe event containing only allowed metadata', () => {
    expect(parseSafeEngineEvent(validEvent, 'tenant-123')).toEqual(validEvent);
  });

  it('rejects an event for a tenant other than the authenticated tenant', () => {
    expect(() => parseSafeEngineEvent(validEvent, 'tenant-456')).toThrow(/tenant/i);
  });

  it('rejects password fields case-insensitively', () => {
    expect(() => parseSafeEngineEvent({ ...validEvent, Password: 'captured-value' }, 'tenant-123')).toThrow(
      /forbidden/i,
    );
  });

  it('rejects credential fields', () => {
    expect(() => parseSafeEngineEvent({ ...validEvent, credential: 'captured-value' }, 'tenant-123')).toThrow(
      /forbidden/i,
    );
  });

  it('rejects unknown fields', () => {
    expect(() => parseSafeEngineEvent({ ...validEvent, browser: 'example' }, 'tenant-123')).toThrow(/unknown/i);
  });

  it('rejects unsupported event types', () => {
    expect(() => parseSafeEngineEvent({ ...validEvent, type: 'form_value_captured' }, 'tenant-123')).toThrow(
      /type/i,
    );
  });

  it('rejects invalid ISO timestamps', () => {
    expect(() => parseSafeEngineEvent({ ...validEvent, occurredAt: 'not-a-timestamp' }, 'tenant-123')).toThrow(
      /timestamp/i,
    );
  });
});
