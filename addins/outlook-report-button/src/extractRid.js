// Extracts an AwareNow (GoPhish-style) tracking rid from arbitrary text (e.g. an
// email body). Mirrors the pattern used elsewhere in this repo — see
// imap/monitor.go's `goPhishRegex` — but simplified for this add-in's needs:
// we only need to recognize a plain `?rid=` / `&rid=` query parameter in body
// text, not the quoted-printable ("=3D") or URL-encoded ("%3D"/"%3F") variants
// that monitor.go has to handle because it parses raw, possibly
// quoted-printable-encoded, email source. Office.js hands us already-decoded
// plain text via item.body.getAsync(Office.CoercionType.Text, ...), so those
// extra encodings don't apply here.
//
// The rid itself is always exactly 7 alphanumeric characters (matches
// GoPhish's tracking ID convention, and imap/monitor.go's `{7}` requirement).
const RID_PATTERN = /[?&]rid=([A-Za-z0-9]{7})\b/;

/**
 * @param {string} text
 * @returns {string|null} the 7-character rid if found, otherwise null.
 */
export function extractRid(text) {
  if (typeof text !== "string") {
    return null;
  }
  const match = RID_PATTERN.exec(text);
  return match ? match[1] : null;
}
