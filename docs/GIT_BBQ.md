# Git BBQ

Git BBQ is the Go-first scaffolding and lifecycle layer around Matt Pocock's
architecture skills. The division of responsibility is intentionally narrow:

- Matt skills own domain language, architecture reasoning, and the canonical ADR
  and context conventions.
- Git BBQ owns project bootstrap, language-specific agent skills, Git policy,
  deterministic projections, validation, and Codex lifecycle hooks.
- Codex remains the host-native reasoning runtime; Git BBQ does not call another
  model or replace Matt's semantic interview.

## Project contract

`git-bbq init` writes operational state and the agent-facing router. A normal
project contains:

| Path | Role |
| --- | --- |
| `CONTEXT.md` | Matt-native project vocabulary and domain language |
| `CONTEXT-MAP.md` | Root context map for future context-specific vocabulary |
| `docs/adr/NNNN-slug.md` | The only architectural decision source of truth |
| `.githabits.yaml` | Granular policy for init, branch, stage, commit, tag, remote, and push |
| `.gitbbq-manifest.yaml` | Project problem, languages, pinned Matt dependency, and hooks |
| `AGENTS.md` / `CLAUDE.md` | Thin agent routers |
| `.agents/skills/` | Githabits plus selected language skills |
| `.agents/mattpocock/DEPENDENCY.yaml` | Upstream repository and commit pin |
| `architecture-contract.yaml` | Generated contract projection |
| `implementation-plan.md` | Generated implementation projection |
| `docs/adr/index.json` | Generated ADR retrieval projection |

The ADR directory is created lazily when the first ADR is recorded, matching the
Matt format. Generated projections can be recreated with `git-bbq project` and
are never a second decision history.

## Existing repositories

Use `git-bbq assess` first. It is read-only and reports proposed paths and
conflicts. `git-bbq apply --approve` is the separate persistence step; it
rechecks the repository before writing. Existing files are preserved unless
`--force` is explicitly supplied.

For a legacy `ai-architect` project, run `git-bbq migrate [path] --json` first.
This command is read-only by default: it inventories `.ai-architect` artifacts,
proposed Matt-native targets, conflicts, and the explicit-approval boundary.
`git-bbq migrate [path] --approve --json` applies an additive migration, keeps
`.ai-architect/`, converts validated ADRs under `docs/adr/`, and archives an
incompatible legacy `.githabits.yaml` before replacing it. It also creates the
contract, implementation-plan, and ADR-index projections without overwriting
pre-existing conflicts. `--force` is required for that legacy-config replacement.

Every new scaffold records hashes in `.gitbbq/ownership.json`. Run
`git-bbq uninstall [path] --json` to review removable files; only
`git-bbq uninstall [path] --approve` removes unchanged files. Modified,
symlinked, reparse-point, and non-regular files or parent paths are reported as
conflicts and preserved. Unowned directories are preserved as well.

Language selection is explicit. Supported profiles are Go, Python, TypeScript,
JavaScript, Rust, Java, and C#. Git profile selection is also explicit: `manual` and
`guided` authorize no Git mutations by default, while `autonomous` enables the
individual action switches until a project changes them.

CLI flags take precedence over project configuration, and project configuration
takes precedence over the optional user-level preference file. Interactive setup
asks whether the selected profile and languages should be remembered. The
`--remember` flag makes that choice explicit in non-interactive use.

The Git mutation seam has a read-only planning step and an explicit execution
step:

```sh
git-bbq githabits plan --action stage --path docs/adr/0001-use-matt-adrs.md --json
git-bbq githabits plan --action init --json
git-bbq githabits plan --action branch --branch feature/git-bbq --json
git-bbq githabits plan --action remote --json
git-bbq githabits plan --action remote --provision-remote --json
git-bbq githabits plan --action commit --message "feat: record architecture decision" --json
git-bbq githabits plan --action tag --tag v0.1.1 --existing-tag v0.1.0 --json
git-bbq githabits plan --action push --branch main --json
git-bbq githabits execute --approve --action init --json
git-bbq githabits execute --approve --action branch --branch feature/git-bbq --json
git-bbq githabits execute --approve --action remote --json
git-bbq githabits execute --approve --action remote --provision-remote --json
git-bbq githabits execute --approve --action commit --message "feat: record architecture decision" --json
git-bbq githabits execute --approve --action tag --tag v0.1.1 --existing-tag v0.1.0 --json
git-bbq githabits execute --approve --action push --branch main --json
```

Plans render argument-separated Git commands and report whether `.githabits.yaml`
authorizes the action. They never execute Git, and every plan still requires
host-level approval. `execute --approve` revalidates the plan, invokes Git without
a shell, applies a bounded timeout, and rejects tampered commands and force
flags. Remote planning reads `remote.alias` and `remote.url` from a pending
`.githabits.yaml` configuration and emits only `git remote add`. After that
approved command succeeds, the CLI persists `remote.status: configured`; a
failed command leaves the status pending. Push planning emits a command only
after the project remote is explicitly marked `configured`; staging rejects
absolute and parent-traversal paths, and conventional-commit profiles reject
messages without a recognized commit type. SemVer profiles require tags in the
`vMAJOR.MINOR.PATCH` form, with optional prerelease and build metadata.
Agents can pass repeated `--existing-tag` values from their read-only tag
inventory; planning rejects duplicate or older release tags.
`--provision-remote` is a separate approval-gated GitHub CLI plan using
`gh repo create --source . --remote <alias>`; it never includes `--push`.

## Codex integration

The plugin package is built from the same Go CLI:

```sh
python3 scripts/build_git_bbq_plugin.py --force
```

The package exposes the structured `Git BBQ` identity and direct `$git-bbq`
skill, and installs exactly five required hooks: `UserPromptSubmit`,
`PreToolUse`, `PostToolUse`, `PostCompact`, and `Stop`. The runtime accepts one
JSON event on standard input and fails closed when the current workspace lacks a
valid Git BBQ manifest or hook configuration.

Generate public contract schemas with:

```sh
go run ./cmd/git-bbq schema .
```

The schemas are written under `schemas/gitbbq/` and are derived from the Go
contracts rather than hand-maintained copies. Planner and executor responses are
available as `schemas/gitbbq/githabits-plan.schema.json` and
`schemas/gitbbq/githabits-execution.schema.json`.

An explicit `git-bbq update --approve --commit <pin>` changes the Matt dependency
pin in both the manifest and its dependency metadata. Validation never fetches or
silently changes the pinned dependency.

## External Matt dependency

New projects record the upstream Matt repository and commit under
`.agents/mattpocock/DEPENDENCY.yaml`. The initial pin is upstream
`c55ee460`; an independently maintained semantic-parity fork can replace it
later through an explicit update workflow. Git BBQ does not silently fetch or
replace a project dependency during validation.
