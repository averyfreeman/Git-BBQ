# Issue tracker: GitHub

Product specs and engineering tickets for this repository live in GitHub
Issues. Use the `gh` CLI from this checkout.

## Operations

- Create an issue with `gh issue create --title ... --body ...`.
- Read an issue with `gh issue view <number> --json number,title,body,labels,comments`.
- List issues with `gh issue list`; filter by state and labels when triaging.
- Apply labels with `gh issue edit <number> --add-label <label>`.
- Specs are parent issues. Link their implementation tickets as native
  sub-issues with `gh issue edit <parent> --add-sub-issue <child>` when the
  repository supports sub-issues.
- Use GitHub's native `blocked by` issue relationships for ticket dependencies.
  With GitHub CLI, create an issue using `--blocked-by <number>` or add the
  relationship later with `gh issue edit <number> --add-blocked-by <number>`.
- If native sub-issues are unavailable, include `Part of #<spec>` in each
  ticket. If native dependencies are unavailable, include a `Blocked by: #N`
  line at the top of the ticket.

## Triage surface

Triage GitHub Issues only. Do not treat pull requests as request submissions.
Use `gh issue` commands for comments, labels, and closure; do not use `gh pr`
commands as part of issue triage.

## Publishing

When a skill asks to publish a spec or ticket, create a GitHub issue. Apply
`ready-for-agent` to a complete spec or ticket unless the work requires human
judgment first. Keep a spec's child tickets dependency ordered and preserve
their native relationships.
