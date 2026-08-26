## Agent skills

### Issue tracker

Issues and specs live in GitHub Issues. See `docs/agents/issue-tracker.md`.

### Triage labels

Uses the default canonical triage labels. See `docs/agents/triage-labels.md`.

### Domain docs

This is a single-context repository. See `docs/agents/domain.md`.

## Code Quality

- Format with `task format`, then lint with `task lint`.
- Follow Staticcheck `ST1005`: error strings, including those passed to `fmt.Errorf`, must start with a lowercase letter and must not end with punctuation or a newline.
