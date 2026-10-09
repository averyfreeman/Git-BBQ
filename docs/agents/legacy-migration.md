# Legacy AI Software Architect migration

Use `git-bbq migrate [path]` for a read-only assessment. Review its legacy
inventory, proposed files, conflicts, blockers, and any required archive
approval before applying it with `git-bbq migrate --approve [path]`.

## Conversion rules

| Legacy source | Converted data | Rule |
| --- | --- | --- |
| `.adr-scaffold.yaml` | `language` | Require metadata version `1`, a supported Git BBQ language, and a non-automatic selection. |
| `.ai-architect/project-context.md` | `motivation.problem` to the Git BBQ manifest problem | Read only validated frontmatter with a compatible context schema. If no problem is available, use the documented migration-purpose fallback. |
| `.ai-architect/decisions/ADR-NNN[-slug].md` | Title, context, decision, rationale, and status as `docs/adr/NNNN-*.md` | Require supported schema/revision, filename and decision-ID agreement, bounded files, and valid required fields. Use `why_this_solution`; if absent, record the migration rationale. |
| `.ai-architect/architecture-contract.yaml`, implementation plan, indexes, and other files | Nothing | Preserve in place. Git BBQ regenerates its own projections from its manifest and Matt-native ADRs. |

Legacy ADR status maps as follows: `accepted` remains accepted, `proposed`
remains proposed, and `rejected`, `deprecated`, and `superseded` become
`deprecated`. Legacy decisions are renumbered after existing Matt-native ADRs;
their original IDs and full structured decision matrices remain in the
preserved legacy tree.

The importer accepts schema versions `1.0.0` and `1.1.0`, at most 200 decision
files, and files no larger than 500 KB. It rejects malformed metadata, unsafe
symlinks, secret-like values in fields it reads, duplicate IDs, unsupported
languages, and incompatible source data before writing.

## Conflicts and preservation

- Existing user files such as `AGENTS.md`, `CONTEXT.md`, and language skills
  are preserved and reported as conflicts; scaffolding skips them.
- An existing Git BBQ manifest, generated projection, incompatible ownership
  ledger, invalid hook configuration, or conflicting Matt dependency pin blocks
  migration. Git BBQ does not overwrite these targets.
- A valid Git BBQ `.githabits.yaml` is preserved. If it is incompatible, the
  assessment marks the archive approval requirement. Apply only after review
  with `--approve --archive-legacy-githabits`; the original is copied to
  `.gitbbq/migration/legacy/.githabits.yaml` before a guided Git BBQ policy
  replaces it. An existing archive target blocks the operation.
- `.adr-scaffold.yaml` and everything under `.ai-architect/` remain unchanged.
  The migration creates additive Git BBQ files and reports any pre-existing
  target paths; it never removes legacy data.

If the assessment lists blockers, resolve them and run it again. The apply
command recalculates the assessment before writing.
