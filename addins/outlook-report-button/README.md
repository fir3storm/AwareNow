# AwareNow Outlook Report-Phishing Add-in

An Office Add-in that implements Microsoft's **integrated spam-reporting**
feature for Outlook, wired to AwareNow's existing report-intake backend. It
replaces Outlook's native "Report" ribbon button with a custom "Report to
AwareNow" button.

This is a v1, deliberately scoped build. See "Known limitations" below for
what's intentionally out of scope.

## What this add-in does

1. Adds a "Report to AwareNow" button in the Outlook ribbon's Report area
   (via the `ReportPhishingCommandSurface` extension point), replacing the
   native Report button.
2. When a user selects it, Outlook shows a simple confirm/cancel
   preprocessing dialog ("This will send a copy of this message to your
   security team for review.").
3. On confirmation, the `SpamReporting` event fires and
   `src/spamreporting.js`'s `onSpamReport` handler runs:
   - Reads the message subject and body text via `Office.context.mailbox.item`.
   - Looks for a 7-character AwareNow/GoPhish tracking `rid` in the body text
     (the same convention `imap/monitor.go`'s `goPhishRegex` uses, e.g.
     `?rid=AbC1234`).
   - **If a rid is found**: `POST {serverUrl}/report?rid=<rid>` — the same
     endpoint GoPhish's own tracking pixel/link hits
     (`controllers/phish.go`'s `ReportHandler`).
   - **If no rid is found** (a real, non-simulated email): `POST
     {serverUrl}/report-unknown` with `{reporter_email, subject, body_text}`
     and a fresh `Idempotency-Key` per attempt — this is
     `controllers/report_intake.go`'s `ReportUnknownHandler`, which stores
     the message in the admin's Reported Messages review queue.
4. Shows a post-processing dialog with a plain success or failure message.

## What this add-in does NOT do

- **No settings UI.** The AwareNow server base URL
  (`awarenowServerUrl`) has no in-add-in configuration screen in v1. It must
  be set once via `Office.context.roamingSettings`, either manually (see
  "One-time server URL setup" below) or pushed centrally by an admin at
  deployment time. This is a known v1 gap — a good fast-follow is a small
  settings task pane or org-wide roaming-settings provisioning.
- **No EML parsing.** It does not use `getAsFileAsync` to fetch the full
  raw MIME message. It only reads `item.subject` and the plain-text body via
  `item.body.getAsync(Office.CoercionType.Text, ...)`. This is simpler and
  requires zero backend changes, since `/report-unknown` already accepts
  exactly `subject` + `body_text` (+ optional `body_html`, which this add-in
  doesn't send).
- **No `JunkFolder` move.** `event.completed` always uses
  `moveItemTo: Office.MailboxEnums.MoveSpamItemTo.NoMove`. This is a
  security-awareness reporting tool used against a mix of real and
  simulated phishing, not a spam-triage tool — moving a message out of the
  inbox isn't appropriate here regardless of report outcome.
- **No reporting-options checkboxes or free-text box** in the preprocessing
  dialog. Kept to a plain confirm/cancel for v1.

## Supported clients

| Client | Status |
| --- | --- |
| Outlook on the web | Supported |
| New Outlook on Windows | Supported |
| Classic Outlook on Windows (2404+ / Build 17530.15000+) | Supported |
| Outlook on Mac (16.100+ / Build 25072537+) | Supported |
| Outlook on Android | **Not supported** |
| Outlook on iOS | **Not supported** |

Android/iOS are not supported because Microsoft's own integrated
spam-reporting feature does not support them on any platform yet — there is
no fallback or workaround available. This is not a gap introduced by this
add-in.

## Files

- `manifest.xml` — add-in-only XML manifest (not the unified JSON manifest,
  which currently only supports classic Outlook on Windows for this
  feature — that would drop web/Mac/new-Windows support). Requires Mailbox
  requirement set 1.14+.
- `src/commands.html` — browser-runtime host page, used by Outlook on the
  web, Mac, and new Outlook on Windows.
- `src/spamreporting.js` — the `SpamReporting` event handler. Loaded as a
  plain classic `<script>` by `commands.html` **and** directly as the
  JS-only runtime override for classic Outlook on Windows. **Contains no
  `import`/`export` statements** — Microsoft's docs state imports aren't
  supported in the file that handles the spam-reporting event on classic
  Outlook on Windows.
- `src/extractRid.js` / `src/extractRid.test.js` — an ES-module, unit-tested
  version of the same rid-extraction regex, kept as the tested source of
  truth. The regex is duplicated (not imported) into `spamreporting.js`
  for the reason above; if you change the pattern, update both files and
  keep the comment in `spamreporting.js` pointing back here.
- `icon-16.png`, `icon-32.png`, `icon-80.png` — **not included in this
  repo**; you must add these three PNG icons before deploying (see
  checklist below).

## One-time server URL setup

Because there's no settings UI yet, an admin must set the server URL once
per client install, e.g. from the browser dev console while the add-in's
`commands.html` is loaded (Outlook on the web: open dev tools on the add-in
iframe):

```js
Office.context.roamingSettings.set('awarenowServerUrl', 'https://your-awarenow-host');
Office.context.roamingSettings.saveAsync();
```

`roamingSettings` follows the user across their devices once set, but there
is no bulk/central way to push this in v1 beyond scripting it once per user
(or documenting it for end users). If unset, the add-in will show a
post-processing dialog telling the reporter it isn't configured, instead of
silently failing.

## Deploy checklist

1. Replace every `REPLACE_WITH_YOUR_DEPLOYMENT_HOST` placeholder in
   `manifest.xml` with your real HTTPS deployment host.
2. Host `src/commands.html`, `src/spamreporting.js`, and the three icon
   files at that host, at the exact paths referenced in `manifest.xml`
   (under `addins/outlook-report-button/...`).
3. Validate the manifest: `npx office-addin-manifest validate manifest.xml`
   (see "Verification" below — this passed full XML Schema validation as
   part of building this add-in).
4. Side-load for testing: Outlook → **Get Add-ins** → **My add-ins** →
   **Add a custom add-in** → **Add from file** → select `manifest.xml`.
5. Org-wide rollout: Microsoft 365 admin center → **Integrated apps** →
   deploy the add-in to targeted users/groups.

## Known limitations

- There is no first-class "report failed" API distinct from
  `event.completed` — the `SpamReporting` event must complete exactly once
  either way (Outlook shows its own 5-minute timeout dialog if it never
  does), so success and failure are both communicated only through the
  post-processing dialog's title/description text, not through a distinct
  error UI.
- No settings UI (see above) — server URL configuration is a manual,
  per-client step in v1.

## REQUIRED: human pilot before this is considered complete

**A controlled mailbox pilot must be run by a human with access to a real
Outlook mailbox before this integration is considered complete.** This
cannot be automated or verified by an AI coding session — it requires an
actual Outlook client, a real or simulated phishing message, and visual
confirmation of Outlook's own dialogs. At minimum, verify:

1. A message containing a recognizable AwareNow rid link (e.g.
   `...?rid=AbC1234...` in the body) routes to `/report`.
2. An unrelated real email with no rid routes to `/report-unknown` and
   appears in the admin Reported Messages review queue.
3. The preprocessing dialog (before reporting) and post-processing dialog
   (after reporting) render correctly and show the expected text.
4. Behavior is confirmed across at least **Outlook on the web** and
   **classic Outlook on Windows** — the two most divergent runtimes (browser
   runtime vs. JS-only runtime with no imports).

## Verification performed while building this add-in

- `npm install && npm test` — 6 `extractRid` unit tests pass (rid extraction
  from `?rid=`/`&rid=` params, no match when absent, and — importantly — no
  false match on a rid-shaped substring embedded in an unrelated longer
  string, or lacking the query-delimiter/word-boundary).
- `npx office-addin-manifest validate manifest.xml` — full XML Schema
  validation passed cleanly (not just well-formedness).
- `node --check src/spamreporting.js` — confirmed syntactically valid, and
  grepped to confirm zero `import`/`export` keywords appear in that file
  (only in code comments explaining why).
- Manually confirmed `src/commands.html` references both `office.js` and
  `spamreporting.js` as plain (non-module) `<script>` tags.
