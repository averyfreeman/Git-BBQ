# Git BBQ glossary

Use these terms consistently in this repository's issue titles, specs, ADRs,
and implementation discussions.

| Term | Meaning |
| --- | --- |
| **Upstream skill source** | The pinned `mattpocock/skills` Git repository recorded in the root curation manifest and `.agents/skills` submodule. |
| **Curation manifest** | Git BBQ-owned metadata selecting which upstream skill directories enter the plugin package. It does not rewrite their names or instructions. |
| **Skill projection** | A selected upstream skill directory copied into the plugin's immediate `skills/<name>/` discovery path. |
| **Metadata adapter** | The narrow packaging step that maps Codex invocation policy into Agent Skills-compatible frontmatter while preserving the upstream policy. |
| **Portable plugin manifest** | Root `plugin.json`, which declares plugin identity and portable package metadata. |
| **Codex compatibility manifest** | `.codex-plugin/plugin.json`, which carries Codex-compatible metadata for hosts that use the fallback manifest. |
| **Generated project context** | Git BBQ's existing `CONTEXT.md` and `CONTEXT-MAP.md` files created for scaffolded repositories. This format is separate from this repository's root glossary. |

Do not use “renamed skill” for a packaged upstream skill. Its package directory
and frontmatter name match the upstream directory name. Do not call the Git
BBQ-owned curation manifest upstream content.
