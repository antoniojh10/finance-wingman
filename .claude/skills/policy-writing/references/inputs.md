# Inputs to collect

Ask in one message. Mark each as given / unknown. Unknown stays a
placeholder in the draft.

## Identity and contact
- Controller: legal name (person or company), country of establishment,
  postal address (say what the user is willing to publish; a business
  normally must publish one).
- Privacy contact email and security/vulnerability contact (role mailboxes
  rather than personal addresses where possible).
- DPO or EU representative: usually none for a small controller; write
  "not appointed" only if the user confirms.
- Supervisory authority for complaints (derived from establishment country).

## Jurisdiction and audience
- Governing law and courts for the terms.
- Who may use the service (invite-only?) and minimum age.
- Free or paid; if paid, the payment processor (none unless the repo says so).

## Data and retention
- Retention per category: account and workspace data, sessions, OAuth
  grants, magic-link tokens, application logs, traces, backups, email
  delivery logs at the email vendor, telemetry at the telemetry vendor.
- Deletion: what happens on account and workspace deletion, and how long
  backups keep deleted data.
- Export: whether a data export exists and its format.

## Vendors
For each: service used, data sent, region, DPA signed or accepted,
transfer mechanism, link to the vendor's own privacy/subprocessor page.
- Hosting (Railway): region of each service and of the database.
- Email (Resend): region of the sending domain; what is stored and for how
  long.
- Telemetry (Grafana Cloud): stack region, attributes sent, retention per
  signal.
- DNS / proxy (Cloudflare): proxied (sees request metadata and IPs) or
  DNS-only.
- AI hosts (Claude, ChatGPT): user-initiated connections, not subprocessors.
- Anything else found in the repo (fonts, analytics, error tracking, CDNs).
  Search before assuming there are none.

## Security statements the user wants to publish
List candidates; each is accepted only if it passes the evidence table.
