---
name: policy-writing
description: Draft or review the privacy policy, terms of service, security page and subprocessor list of finance-wingman (small EU-oriented SaaS), in English and Spanish, with every claim tied to evidence in the repo. Use when the user asks to write, update, audit or translate legal/trust pages, a subprocessor list, a DPA summary, or to check that public claims match what the product ships.
---

# Policy writing

Writes trust and legal pages for a small SaaS that holds personal finance
data. The goal is documents that are accurate, plain-language and
verifiable, not impressive. **Not legal advice**: every draft ships with the
disclaimer in `references/disclaimer.md` in the review notes (never in the
published page), and the owner should have a lawyer review it before
relying on it.

## Hard rules

1. **No claim without evidence.** Each factual statement (encryption,
   hashing, retention, region, rate limits, headers, backups, deletion) must
   map to code, config, a migration, a doc or a vendor page, recorded in an
   evidence table (`references/evidence-mapping.md`). If it is not shipped
   today, leave it out or mention it as "planned" in the review notes only.
   Never publish future-tense promises.
2. **Verify, do not assume.** Regions, retention periods and vendor
   behaviour are not in the repo unless documented. Ask the user or check
   the vendor's current docs and cite the URL and date. Task tickets in
   `.tasks/` (when present) are hints, not proof: confirm the feature is
   merged by reading the code.
3. **No secrets or personal data in the skill or in committed output.**
   Legal name, address, contact email and domains stay placeholders
   (`{{LEGAL_NAME}}`) until the user supplies them.
4. **Plain language.** Short sentences, active voice, say "we". Define terms
   once. See `references/style-en-es.md`.
5. **English first, Spanish second**, same structure and same claims. The
   Spanish version is written natively, not word by word; the evidence
   table is shared.

## Workflow

1. **Collect inputs** with `references/inputs.md` (controller identity,
   contact, jurisdiction, retention, regions, subprocessors). Ask for
   missing values in one batch; leave `{{PLACEHOLDER}}` for what stays
   unknown and list them at the end.
2. **Inventory the facts from the repo** following
   `references/evidence-mapping.md`: data categories, cookies, auth, logs and
   telemetry attributes, third-party calls, deployment docs
   (`docs/deploy-railway.md`, `docs/observability.md`), env vars in
   `apps/api/internal/config`.
3. **Draft** the requested document using the section lists in
   `references/gdpr-checklist.md` (privacy, terms, security page) and
   `references/subprocessors-template.md`. Write content as standalone
   Markdown files (no app layout, no framework imports) so the web app and a
   marketing site can reuse them, one file per language and document.
4. **Review pass**: walk the checklist; every box needs a section or a
   written reason it does not apply. Walk the evidence table; every row
   needs a source. Search the draft for banned phrases (style notes).
5. **Report** to the user: files written, the evidence table, open
   placeholders, claims dropped for lack of evidence, and items that need a
   lawyer (marked `[LEGAL REVIEW]` in the review notes, not in the page).

## Product-specific points

- Data lives in workspaces; users sign in by magic link (no passwords);
  AI hosts (Claude, ChatGPT) connect through MCP/OAuth and act with the
  user's permissions. The privacy policy must explain that when the user
  connects an AI host, that host's provider processes the conversation under
  its own terms; it is a recipient the user chooses, not our subprocessor.
- Money data is personal data but not a special category under GDPR art. 9;
  do not call it "sensitive" in the legal sense, and do not use
  certification language (SOC 2, ISO 27001, PCI) we do not hold.
- Telemetry: state exactly which attributes are exported (confirm against
  `apps/api/internal/telemetry` and `apps/web/src/lib/telemetry*.ts`).
- Railway's region is not recorded in `docs/deploy-railway.md`; ask the
  user to read it from the dashboard. The same goes for Resend, Grafana
  Cloud and Cloudflare settings.
