---
name: ask-matt
description: Ask which packaged ADR, architecture, code, or documentation workflow fits the current repository task.
---

# Ask Matt

Use this router when the right engineering workflow is unclear. Choose the
narrowest packaged skill that produces the artifact or decision the repository
needs.

## Decision map

- **Create or revise an ADR or domain glossary** → `$grill-with-docs`, then
  `$domain-modeling` as terminology or decisions crystallize.
- **Revise architecture** → `$improve-codebase-architecture` to surface
  candidates, `$codebase-design` to shape the chosen module, and `$prototype`
  when a state model or interface needs a runnable answer.
- **Implement a known change** → `$tdd`, then `$implement`, then `$code-review`.
- **Diagnose a hard bug or regression** → `$diagnosing-bugs`; escalate to
  `$improve-codebase-architecture` when the missing seam is the finding.
- **Investigate an external technical question** → `$research`, then carry
  the cited findings into `$grill-with-docs` or an ADR.
- **Resolve an in-progress merge or rebase** → `$resolving-merge-conflicts`.
- **Revise agent-facing documentation or a skill** → `$writing-for-agents`.
- **Stress-test a plan without recording documents yet** → `$grilling`.

## Shared vocabulary

Use `$codebase-design` for module, interface, depth, seam, adapter, leverage,
and locality. Use `$domain-modeling` for project terminology, `CONTEXT.md`, and
architectural decisions. Keep `docs/adr/` as the decision source of truth.

## Completion

When a route produces a durable decision, capture it in the repository's ADR or
context documents. When it produces code, finish with tests and `$code-review`.
