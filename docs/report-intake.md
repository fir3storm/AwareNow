# Unknown-message reporting

Set `phish_server.report_owner_id` in `config.json` to an existing local user
ID and restart the Go service to enable `POST /report-unknown`. The default
`0` disables intake (HTTP 503). A missing configured user also returns 503.
All reports received by this engine belong to that configured user. Review
APIs derive the owner from the authenticated user and require the existing
modify-objects permission. Other users cannot list, read, approve, or reject
those reports. Changing the intake owner affects new reports only.

This is a local engine/user binding, not a shared tenant identifier. In a
tenant deployment, provisioning must bind the engine and local owner to its
trusted tenant configuration, consistent with the
[control-plane contracts](architecture/control-plane-contracts.md). The public
endpoint never accepts an owner or tenant from the request. Reporter email is
an unverified claim, not proof of identity; CORS does not authenticate callers.

Send JSON with `reporter_email`, optional `subject`, and at least one nonblank
`body_text` or `body_html`. Unknown fields and trailing JSON are rejected.
The total request limit is 1 MiB, the email limit is 254 bytes, and the subject
limit is 998 bytes with no line breaks. Invalid fields return 400; unsupported
content types return 415; oversized requests return 413. Successful intake
returns 204. The existing per-IP limiter permits five POSTs per minute and
returns 429 when exhausted. OPTIONS preflights consume no POST budget; CORS
headers are included on failures as well as successful requests.

Clients should generate an `Idempotency-Key` per report and reuse it for
retries, keeping the email and content unchanged. Keys may be at most 128
bytes and cannot have leading or trailing whitespace. Deduplication is scoped
to the configured owner, exact reporter email (including case), and key.
An identical retry returns 204 without creating another report; changed
content with the same scoped key returns 409. Distinct reporters are never
merged. Keys persist with the report; without a key each request is new.

On startup, GORM adds owner and nullable unique idempotency-hash columns to
existing report tables. Historical reports with NULL or zero owners stay
quarantined: no review API exposes them, including to administrators. There
is no automatic ownership inference from email. If recovery is needed, an
operator must back up the database, establish the legitimate owner from
trusted records, and explicitly assign only those verified report IDs to
that existing user's `owner_id`. Leave uncertain ownership quarantined.

Approval creates a template and changes pending status in one transaction.
Rejection also requires pending status. Repeated or competing decisions
return 409; foreign-owner and missing report IDs return 404. Failed template
creation leaves the report pending and creates no orphan template.
