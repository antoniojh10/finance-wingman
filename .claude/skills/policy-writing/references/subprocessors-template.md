# Subprocessor list template

Publish as Markdown. Replace every `{{...}}` with a verified value and keep
the source URL and check date in the review notes. Keep only vendors that
actually receive personal data from the product.

| Vendor | Purpose | Personal data involved | Processing location | Transfer mechanism | DPA |
|---|---|---|---|---|---|
| Railway | Hosting of the API, web app and database | Account and workspace data; IP addresses in request logs | {{REGION from dashboard}} | {{SCCs / adequacy, vendor page}} | {{link}} |
| Resend | Sending sign-in and notification emails | Recipient email, message content, delivery events | {{REGION of the sending domain}} | {{...}} | {{link}} |
| Grafana Cloud | Traces, metrics and logs to operate the service | Only the attributes listed in the privacy policy | {{stack region}} | {{...}} | {{link}} |
| Cloudflare | DNS {{and proxy, if enabled}} | Domain queries {{and request metadata if proxied}} | {{location}} | {{...}} | {{link}} |

## Not subprocessors (explain separately)

AI hosts such as Claude and ChatGPT. The user connects them to their own
workspace over MCP/OAuth; what the host receives is governed by the host's
terms and the user's choice. Say what the tools return (accounts,
transactions, summaries) and that access can be revoked (only if a
revocation UI exists; check first).

## Change policy text (adapt)

"We update this list before we start using a new vendor that handles
personal data. The date at the top shows the last change."
Promise a notice period or notification only if the user commits to one.
