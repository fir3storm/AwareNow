# USP-1: Assessment Lab Measurement Specification

Status: draft, not yet implemented. Defines the metrics referenced in the
Phase 1 plan's "Product USP — AwareNow Proof of Resilience" section
(`docs/superpowers/plans/2026-09-05-phishing-assessment-phase1.md`) precisely
enough to implement and to validate against hand-checked fixtures, per that
plan's USP-1 requirement. This spec does not authorize implementation beyond
USP-1 itself; USP-2 onward still require their own work.

Grounded in the actual current data model (verified against source, not
assumed): `models.Campaign` (`Events []Event`), `models.Result` (per-recipient
current state: `Status`, `SendDate`, `Reported bool`, `TimeToClickMs`,
`RiskLevel`, ...), `models.Event` (`{CampaignId, Email, Time, Message,
Details}`, one row per state transition), and the status-string constants in
`models/models.go`: `EventSent = "Email Sent"`, `EventOpened = "Email Opened"`,
`EventClicked = "Clicked Link"`, `EventDataSubmit = "Submitted Data"`,
`EventReported = "Email Reported"`.

None of `Assessment`, `Scenario`, or `Cohort` exist yet — they are USP-2's
job. This spec defines the metrics in terms of what USP-2 must be able to
answer (which campaign(s)/result(s) belong to which assessment cohort, which
scenario variant, whether it's a threat or benign control), without
prescribing USP-2's schema itself.

## 1. Core concepts

- **Assessment**: one measurement exercise with a baseline phase and,
  optionally, a follow-up phase, each using a distinct scenario variant so
  follow-up measures transfer of a skill rather than memorization of the
  exact original message.
- **Scenario**: a sanitized, versioned message template tagged with exactly
  one **skill** being tested (MVP boundary: one recognition skill per
  assessment) and a **kind**: `threat` (simulated phishing) or `benign`
  (a legitimate-looking control message that should NOT be reported as
  phishing).
- **Cohort**: the set of recipients assigned to one assessment phase. A
  recipient belongs to at most one cohort per assessment phase (no overlap
  within a phase; the same person MAY appear in both the baseline and
  follow-up cohorts of the same assessment — that repeat is expected and is
  the basis of the before/after comparison).
- **Observation window**: a fixed duration, set once per assessment before
  it launches, starting at each recipient's individual `SendDate` (not a
  single wall-clock cutoff for the whole cohort — recipients are not all
  sent their message at the same instant). Events outside a recipient's own
  window do not count toward that recipient's metrics.
- **Risky interaction**: for a `threat` scenario, the first `EventClicked` or
  `EventDataSubmit` event (see §2 for ordering) for that recipient in that
  assessment phase. Benign scenarios have no risky interaction — clicking a
  benign control is not risky, so Recognition/Recovery are not computed for
  benign cohorts (see §3.2, Discrimination instead).

## 2. Event ordering and deduplication

Every metric below reads a recipient's `Event` rows for the relevant
campaign, filtered to `Email` = that recipient and `Time` within their
observation window, sorted ascending by `Time`.

- **Canonical severity order** (for tie-breaking only, not for filtering):
  `Sent (0) < Opened (1) < Clicked (2) < DataSubmit (3) < Reported (4)`.
- **Tie-breaking rule**: if two events for the same recipient carry the
  identical `Time` value (millisecond collisions are possible from batch
  processing), order by the canonical severity above, not insertion order.
  This is a rare edge case; document it in code with a comment rather than
  silently relying on database row order, which is not guaranteed stable.
- **Deduplication**: a recipient may have multiple `Opened`/`Clicked` events
  (e.g. re-opening the email, clicking twice). For every metric in this
  spec, use only the **first** occurrence of each event type per recipient
  per assessment phase. Repeated interactions do not multiply a recipient's
  contribution to any metric — this is a per-person binary/first-timestamp
  measurement, not an interaction count. (Raw per-recipient counts like
  `Result.TotalClicks` remain available for operational/debugging views;
  they are out of scope for the evidence report itself.)
- **First risky interaction** := `min(Time)` across that recipient's first
  `Clicked` event and first `DataSubmit` event, if any exist within window.
- **First report** := that recipient's first `Reported` event `Time` within
  window, if any.

## 3. Metrics

Every metric is reported per assessment phase (baseline or follow-up) per
cohort, never pooled across phases (pooling would hide the before/after
comparison that is the entire point).

### 3.1 Recognition (threat scenarios only)

> Share of eligible simulated-threat recipients who report before a defined
> risky interaction within the observation window.

- **Eligible** := recipient has a `Sent` event in this phase (i.e. the send
  was attempted — see §4 on delivery-confirmation uncertainty) AND is a
  member of a `threat`-scenario cohort for this phase. A recipient with no
  `Sent` event at all (send failed before dispatch) is excluded from the
  denominator entirely — that is a delivery failure, not a behavioral
  observation.
- **Recognized** (numerator) := eligible recipient whose first `Reported`
  event exists AND (no risky interaction exists, OR first `Reported` time <
  first risky interaction time).
- **Recognition rate** = Recognized / Eligible, as a proportion in `[0,1]`.
- A recipient who neither reported nor had a risky interaction (opened and
  did nothing, or never opened) is **eligible but not recognized** — counted
  in the denominator, not the numerator. Per the plan's explicit
  instruction, nonresponse is never silently treated as safe/positive
  behavior; it lowers the recognition rate exactly as clicking would, and
  is additionally surfaced as its own separate figure (§3.5) so a reviewer
  can distinguish "actively failed" from "did not engage" — these have
  different remediation implications even though both currently count the
  same way in this specific proportion.

### 3.2 Discrimination (benign scenarios only)

> Benign-control reporting rate, displayed separately from threat
> reporting; never used to discourage reporting uncertain real mail.

- **Eligible** := recipient has a `Sent` event AND is a member of a
  `benign`-scenario cohort for this phase.
- **Reported-as-threat** (numerator) := eligible recipient has a `Reported`
  event within window (there is no "risky interaction" concept for benign
  mail, so this is a simple report-or-not proportion, not a race).
- **Discrimination rate** = Reported-as-threat / Eligible.
- This number is expected to be non-zero and is not itself a failure —
  over-reporting benign mail is a far safer error than under-reporting
  threats. The evidence report must present it neutrally (§3.5's uncertainty
  interval applies here too) and must never be combined into a single
  composite "score" with Recognition — the plan explicitly prohibits an
  opaque individual risk score, and combining these two into one number
  would recreate that problem at the cohort level.

### 3.3 Recovery (threat scenarios only)

> Reports made after a risky interaction, counted separately from early
> detection so improvement is visible without rewriting the event history.

- **Eligible** := same as Recognition's eligible set, restricted further to
  recipients who had a risky interaction (i.e. were NOT recognized early).
- **Recovered** (numerator) := eligible recipient whose first `Reported`
  event exists and its time is >= first risky interaction time (reported
  after the fact) — this is exactly the complement condition to Recognition
  within the "had a risky interaction" subset, so Recognition + non-report
  + Recovery partition that subset without overlap.
- **Recovery rate** = Recovered / Eligible (this eligible set, not the full
  cohort). Report the absolute count alongside the rate — a 100% recovery
  rate over 2 people reads very differently from 100% over 200.

### 3.4 Speed (threat scenarios only)

> Time-to-report distribution plus the nonreporting proportion; distinguish
> SMTP acceptance from confirmed delivery when delivery is unknown.

- For every eligible recipient (as in Recognition) with a `Reported` event,
  compute `time_to_report = first Reported time - SendDate`.
- Report the distribution as median and interquartile range (25th/75th
  percentile), not a mean alone — reporting-time distributions are heavily
  right-skewed in practice (most people who report do so quickly; a long
  tail of slow reporters would distort a mean). Use a documented,
  deterministic percentile method (e.g. linear interpolation between
  ranks) so results are reproducible from the same input set.
- **Nonreporting proportion** = (Eligible − count with any `Reported`
  event) / Eligible. This is 1 minus Recognition's numerator rate only if
  Recovery is zero; report it independently rather than deriving it, since
  a reviewer should be able to sanity-check consistency between the two.
- **Delivery-confirmation caveat**: `SendDate` marks when this engine
  attempted SMTP submission, not confirmed inbox delivery (there is no
  read-receipt or delivery-status-notification integration in this
  codebase today). Time-to-report is therefore measured from send attempt,
  and the evidence report must display this as a labeled assumption (e.g.
  "time measured from send attempt; delivery confirmation is not
  available") rather than silently implying confirmed delivery. Do not
  infer delivery from an `Opened` event either — pixel-based open tracking
  is known to under-count (image-blocking clients) and must not be used as
  a proxy for "was delivered."

### 3.5 Evidence quality (reported alongside every metric above, every phase)

- Cohort size (eligible count) and total assigned count (before any
  exclusions), with a plain-language reason for every exclusion category
  (e.g. "excluded: send failed", "excluded: duplicate assignment").
- Scenario version identifier(s) in play for this phase.
- The **nonresponse count** referenced in §3.1 (eligible, no report, no
  risky interaction) reported as its own labeled figure, not merged
  silently into "not recognized."
- **Uncertainty interval**: report a 95% Wilson score interval (not a naive
  normal approximation, which misbehaves near 0% or 100% — exactly the
  range small pilot cohorts often produce) for every proportion in §3.1-3.4.
  Formula, for `n` trials and `p̂` observed proportion, `z = 1.959964`
  (95%):
  ```
  center = p̂ + z²/(2n)
  half_width = z * sqrt( p̂(1-p̂)/n + z²/(4n²) )
  denom = 1 + z²/n
  interval = ( (center - half_width) / denom, (center + half_width) / denom )
  ```
- **Small-cohort suppression threshold**: if `n < 10` (eligible count) for a
  given metric/phase/cohort combination, do not display a computed
  percentage at all — display "insufficient data (n=<count>)" instead. This
  threshold is a starting default, not empirically validated yet; USP-5's
  buyer pilot may reveal it should be higher. Record it as a named,
  overridable constant in code (not a magic literal repeated at call
  sites), specifically so it can be revisited without a scattered find-edit.
- **Suspected scanner activity**: security scanners and link-preview
  services sometimes trigger opens/clicks automatically without human
  involvement, which would corrupt Recognition/Recovery if counted as
  genuine risky interactions. USP-1 does not define scanner-detection
  heuristics (that's explicitly a `models.Result` concern already —
  `EmailClient`/`DeviceType`/`Referrer`/`TLSVersion` are captured per
  result); USP-4 must decide which of those fields feed a
  "suspected-automated" label and must report raw-versus-filtered results
  side by side, per the plan's explicit requirement, rather than silently
  dropping suspected-scanner rows.

## 4. Worked fixture (hand-checked, for the first implementing test to assert against)

A `threat` scenario, one phase, 12 assigned recipients, all successfully
sent (12 eligible), observation window 72 hours from each `SendDate`.

| Recipient | Sent (h=0) | First event after Sent | Time (h) | Reported? | Time (h) |
|---|---|---|---|---|---|
| r1 | ✓ | Reported | 1 | — | — |
| r2 | ✓ | Reported | 4 | — | — |
| r3 | ✓ | Clicked | 2 | yes (recovered) | 10 |
| r4 | ✓ | Clicked | 0.5 | no | — |
| r5 | ✓ | DataSubmit | 1 | no | — |
| r6 | ✓ | Opened | 3 | no further event | — |
| r7 | ✓ | (nothing) | — | — | — |
| r8 | ✓ | Reported | 0.25 | — | — |
| r9 | ✓ | Clicked | 20 | yes (recovered) | 30 |
| r10 | ✓ | Reported | 50 | — | — |
| r11 | ✓ | Clicked | 71 | no (window closes at 72) | — |
| r12 | ✓ | Reported at 80h (outside 72h window) | — | excluded (outside window) | — |

Expected computed values (n = 12 eligible; r12's late report is outside the
window and does not count as a report at all for this phase — treat it as
"no report within window", i.e. r12 behaves like r7/r11 for Recognition):

- **Recognized** = {r1, r2, r8, r10} = 4. **Recognition rate** = 4/12 =
  0.3333 (33.33%).
- **Risky-interaction subset** (had a risky interaction: r3, r4, r5, r9,
  r11) = 5 recipients.
  - Of these, **Recovered** (reported after risky interaction, within
    window) = {r3, r9} = 2. **Recovery rate** = 2/5 = 0.40 (40%).
  - r4, r5, r11 had a risky interaction and never reported within window:
    3 recipients, not recovered.
- **Nonresponse** (eligible, no report, no risky interaction) = {r6, r7,
  r12} = 3. (r12 counts here, not as a report, because its report fell
  outside the window.)
- Sanity check: Recognized (4) + risky-interaction subset (5) + nonresponse
  (3) = 12 = eligible count. Every eligible recipient falls into exactly
  one bucket.
- **Time-to-report distribution** (recognized + recovered reports, times in
  hours): {1, 4, 0.25, 50} ∪ {10, 30} = {0.25, 1, 4, 10, 30, 50}, n=6.
  Sorted: 0.25, 1, 4, 10, 30, 50. Median (linear interpolation, rank
  (n-1)/2 = 2.5) = (4 + 10)/2 = 7 hours. 25th percentile (rank 0.25*5=1.25)
  = 1 + 0.25*(4-1) = 1.75 hours. 75th percentile (rank 0.75*5=3.75) = 10 +
  0.75*(30-10) = 25 hours.
- **Nonreporting proportion**: recipients with any `Reported` event within
  window = {r1, r2, r3, r8, r9, r10} = 6 (this includes r3 and r9, whose
  reports came after their risky interaction — "any report", not "early
  report"). Nonreporting = 12 − 6 = 6, proportion = 6/12 = 0.50 (50%). This
  matches the time-to-report distribution's n=6 above — internally
  consistent, as required by §3.4's cross-check.
- **95% Wilson interval for Recognition rate** (p̂=4/12=0.3333, n=12),
  computed with the §3.5 formula (z=1.959964): **(0.1381, 0.6094)**
  (verified by script, not hand arithmetic — an implementing test must
  match this to 4 decimal places: 0.1381 and 0.6094). Given n=12 this is
  very wide, correctly reflecting low confidence at this cohort size — and
  per §3.5, n=12 clears the suppression threshold (>=10) so it displays,
  but the wide interval itself is the honest signal that more data is
  needed, which is the point of reporting it at all.

An implementing test should build exactly these 12 fixture recipients'
`Result`/`Event` rows (or the eventual `Assessment`/`Cohort`-scoped
equivalent once USP-2 exists) and assert every number above to 4 decimal
places, including the Wilson interval bounds.

## 5. Explicitly out of scope for USP-1

- Any UI, API endpoint, or persisted schema (USP-2/USP-3/USP-4).
- Scanner-detection heuristics themselves (flagged for USP-4 to design,
  using existing `Result` fields).
- Cross-assessment or cross-organization benchmarking.
- Any claim of causality between an intervention and observed change — this
  spec only defines observational metrics; the plan is explicit that
  "a before/after change alone is observational; causal wording requires a
  justified comparison design," which is a product-copy/UX concern for
  USP-4, not a metrics-definition concern here.
