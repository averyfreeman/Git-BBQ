---
name: find-grill
description: Ask which Git BBQ v0.3 skill fits the current repository task.
policy.allow_implicit_invocation: false
---

# Find grill

Use this router when the right Git BBQ workflow is unclear. Choose the
narrowest final v0.3 entrypoint that produces the needed decision, artifact, or
change.

## Decision map

- **Sharpen a repository plan and update its vocabulary or ADRs** →
  `$grill-for-docs`.
- **Sharpen a plan without repository documents** → `$grill-no-docs`.
- **Implement a known change** → `$fire-away`; use `$test-first` for a focused
  red-green slice and `$review-code` for a review.
- **Diagnose a hard bug or regression** → `$debug`.
- **Design a module, interface, or seam** → `$design-project`.
- **Prototype a state model or UI** → `$prototype`.
- **Investigate a technical question and save cited findings** → `$research`.
- **Resolve an in-progress merge or rebase conflict** →
  `$fix-merge-or-rebase`.
- **Strengthen project language, glossary, or ADR vocabulary** →
  `$strengthen-vocabulary`.
- **Recover from a misunderstood request** → `$clarify`.
- **Write or revise an agent-facing skill or instruction** →
  `$write-agent-docs`.
- **Review a branch, work-in-progress, or change against a fixed point** →
  `$review-code`.
- **Create a session or environment retrospective** →
  `$create-retrospective`.

## Completion

Route to one final v0.3 entrypoint. If the selected workflow invokes another
entrypoint, use only the canonical names listed above and preserve that
workflow's stated report or artifact contract.
