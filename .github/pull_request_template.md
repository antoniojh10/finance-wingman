## Summary

<!-- What changes and why. Link the issue if there is one. -->

## Tests run

- [ ] `make api-lint api-test` (backend changes)
- [ ] `make web-lint web-test` (frontend changes)
- [ ] `make web-e2e` (user flow changes)
- [ ] Tests added or updated for the change

## Checklist

- [ ] Commits follow Conventional Commits
- [ ] `make api-generate` / `make api-openapi` run if SQL or API schemas changed
- [ ] New schema changes are in a new migration with a working `Down`
- [ ] UI copy added to both `messages/en.json` and `messages/es.json`
