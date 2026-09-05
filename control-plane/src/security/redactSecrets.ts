const redactedValue = '[REDACTED]';
const credentialKeyPattern = /(password|secret|token|credential|apikey|authorization)/i;

export function redactSecrets(value: unknown): unknown {
  return redactValue(value, new WeakMap<object, unknown>());
}

function redactValue(value: unknown, seen: WeakMap<object, unknown>): unknown {
  if (Array.isArray(value)) {
    const existing = seen.get(value);
    if (existing !== undefined) {
      return existing;
    }

    const redacted: unknown[] = [];
    seen.set(value, redacted);
    for (const item of value) {
      redacted.push(redactValue(item, seen));
    }
    return redacted;
  }

  if (!isPlainRecord(value)) {
    return value;
  }

  const existing = seen.get(value);
  if (existing !== undefined) {
    return existing;
  }

  const redacted: Record<string, unknown> = {};
  seen.set(value, redacted);

  for (const [key, nestedValue] of Object.entries(value)) {
    redacted[key] = credentialKeyPattern.test(key) ? redactedValue : redactValue(nestedValue, seen);
  }

  return redacted;
}

function isPlainRecord(value: unknown): value is Record<string, unknown> {
  if (typeof value !== 'object' || value === null) {
    return false;
  }

  const prototype = Object.getPrototypeOf(value);
  return prototype === Object.prototype || prototype === null;
}
