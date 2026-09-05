export type SafeEventType =
  | 'email_sent'
  | 'email_opened'
  | 'link_clicked'
  | 'landing_page_viewed'
  | 'form_submitted'
  | 'report_submitted'
  | 'training_completed'
  | 'campaign_completed';

export type SafeEngineEvent = {
  eventId: string;
  tenantId: string;
  campaignId: string;
  recipientRef: string;
  type: SafeEventType;
  occurredAt: string;
  fieldNames?: string[];
  fieldCount?: number;
};

const allowedKeys = new Set<keyof SafeEngineEvent>([
  'eventId',
  'tenantId',
  'campaignId',
  'recipientRef',
  'type',
  'occurredAt',
  'fieldNames',
  'fieldCount',
]);

const forbiddenKeys = new Set(['password', 'secret', 'token', 'credential', 'value']);

const allowedTypes = new Set<SafeEventType>([
  'email_sent',
  'email_opened',
  'link_clicked',
  'landing_page_viewed',
  'form_submitted',
  'report_submitted',
  'training_completed',
  'campaign_completed',
]);

const identifierPattern = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,255}$/;
const timestampPattern = /^(\d{4})-(\d{2})-(\d{2})T(?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d(?:\.\d{1,9})?(?:Z|[+-](?:[01]\d|2[0-3]):[0-5]\d)$/;

type UnknownRecord = Record<string, unknown>;

export function parseSafeEngineEvent(input: unknown, authenticatedTenantId: string): SafeEngineEvent {
  if (!isRecord(input)) {
    throw new Error('Safe event must be an object.');
  }

  for (const key of Object.keys(input)) {
    if (forbiddenKeys.has(key.toLowerCase())) {
      throw new Error(`Forbidden event field: ${key}`);
    }

    if (!allowedKeys.has(key as keyof SafeEngineEvent)) {
      throw new Error(`Unknown event field: ${key}`);
    }
  }

  const eventId = parseIdentifier(input.eventId, 'eventId');
  const tenantId = parseIdentifier(input.tenantId, 'tenantId');
  const campaignId = parseIdentifier(input.campaignId, 'campaignId');
  const recipientRef = parseIdentifier(input.recipientRef, 'recipientRef');
  const type = parseEventType(input.type);
  const occurredAt = parseTimestamp(input.occurredAt);

  if (tenantId !== authenticatedTenantId) {
    throw new Error('Event tenant does not match the authenticated tenant.');
  }

  const event: SafeEngineEvent = {
    eventId,
    tenantId,
    campaignId,
    recipientRef,
    type,
    occurredAt,
  };

  if (input.fieldNames !== undefined) {
    event.fieldNames = parseFieldNames(input.fieldNames);
  }

  if (input.fieldCount !== undefined) {
    event.fieldCount = parseFieldCount(input.fieldCount);
  }

  return event;
}

function isRecord(value: unknown): value is UnknownRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseIdentifier(value: unknown, name: string): string {
  if (typeof value !== 'string' || !identifierPattern.test(value)) {
    throw new Error(`Invalid ${name}.`);
  }

  return value;
}

function parseEventType(value: unknown): SafeEventType {
  if (typeof value !== 'string' || !allowedTypes.has(value as SafeEventType)) {
    throw new Error('Invalid event type.');
  }

  return value as SafeEventType;
}

function parseTimestamp(value: unknown): string {
  if (typeof value !== 'string' || !timestampPattern.test(value) || !isCalendarDate(value)) {
    throw new Error('Invalid event timestamp.');
  }

  return value;
}

function isCalendarDate(timestamp: string): boolean {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(timestamp);
  if (match === null) {
    return false;
  }

  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const daysInMonth = new Date(Date.UTC(year, month, 0)).getUTCDate();

  return month >= 1 && month <= 12 && day >= 1 && day <= daysInMonth;
}

function parseFieldNames(value: unknown): string[] {
  if (
    !Array.isArray(value) ||
    value.some((fieldName) => typeof fieldName !== 'string' || fieldName.trim().length === 0 || fieldName.length > 128)
  ) {
    throw new Error('Invalid event field names.');
  }

  return [...value];
}

function parseFieldCount(value: unknown): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) {
    throw new Error('Invalid event field count.');
  }

  return value;
}
