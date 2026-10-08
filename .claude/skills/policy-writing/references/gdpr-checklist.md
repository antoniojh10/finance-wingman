# GDPR checklists

For a small controller serving EU users. Tick each item or write why it
does not apply. A drafting aid, not legal advice.

## Privacy policy (arts. 12-14 GDPR)

- [ ] Who we are: controller identity, address, contact; DPO or
      representative if any (or "not appointed").
- [ ] Scope: web app, API, MCP connector.
- [ ] Data we hold by category and source: account (email, name),
      workspace data (accounts, transactions, categories, budgets,
      recurring items, balances), technical data (IP, user agent, session
      and OAuth records), email delivery data, telemetry.
- [ ] Data we do not collect (only if verified): payment cards, bank
      credentials, third-party trackers.
- [ ] Purpose and legal basis per purpose (art. 6): provide the service
      (contract), security and abuse prevention (legitimate interest, name
      the interest), legal obligations, consent only where really used.
- [ ] Recipients and subprocessors (link to the list); AI hosts the user
      connects.
- [ ] Transfers outside the EEA: which vendors, which country, which
      mechanism (adequacy decision, SCCs, EU-US Data Privacy Framework);
      cite the vendor's DPA page. If none, say so only after verifying.
- [ ] Retention per category, with trigger and period.
- [ ] Rights: access, rectification, erasure, restriction, portability,
      objection, withdrawing consent; how to exercise them (self-service
      export/delete only if shipped, else email) and the one-month reply.
- [ ] Right to complain to a supervisory authority (name it).
- [ ] Whether providing data is required, and the consequence.
- [ ] Automated decision-making: none, if true (suggestions from recurring
      detection have no legal effect; confirm).
- [ ] Cookies and local storage: each one (name, purpose, duration);
      strictly necessary ones need no banner.
- [ ] Security measures in brief, linking to the security page.
- [ ] Children: minimum age.
- [ ] Data breach: how affected users are told.
- [ ] Changes to the policy: how users are notified.
- [ ] Effective date and version.

## Terms of service

- [ ] Parties and acceptance; who can use the service (invite-only).
- [ ] What the service is and is not (a tracker, not financial advice, not
      a bank; balances are as entered).
- [ ] Accounts, sign-in links, workspace roles and owner duties.
- [ ] AI integrations: third-party AI hosts run under their own terms;
      tool actions run with the user's permissions; the user reviews what
      an AI changes.
- [ ] Acceptable use and suspension.
- [ ] User content stays theirs; our licence is limited to running the
      service.
- [ ] Availability: best effort; changes and ending the service with
      notice and an export window.
- [ ] Liability limits and warranty disclaimers (mandatory consumer rights
      cannot be waived: `[LEGAL REVIEW]`).
- [ ] Termination by the user (delete account) and by us.
- [ ] Governing law and courts, with consumer-forum carve-out.
- [ ] Changes to terms; contact.

## Security page

- [ ] Only controls with a `shipped` or `config` row in the evidence table.
- [ ] Mechanism and scope in one or two sentences each; no adjectives such
      as "military-grade", "unhackable", "bank-level".
- [ ] State limits when a reader would assume otherwise (for example no
      field-level encryption, no third-party audit).
- [ ] Vulnerability reporting address and response time (only if the user
      commits).
- [ ] Incident notification approach.
- [ ] Links to the subprocessor list and privacy policy.
- [ ] Date of last review.

## Subprocessor list

- [ ] Table from `subprocessors-template.md`, one row per vendor, each cell
      verified or `{{TO CONFIRM}}`.
- [ ] How users learn of changes.
- [ ] DPA status for each vendor.

## Controller / processor note

Workspace owners entering household members' data may fall under the
household exemption (art. 2(2)(c)). We are controller for account and
technical data. Do not write a processor DPA unless asked; flag it in the
review notes.
