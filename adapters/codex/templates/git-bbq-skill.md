---
name: git-bbq
description: Scaffold and maintain an agent-ready repository with Matt-native architecture records and explicit Git habits.
---

# Git BBQ

Use the `git-bbq` executable for repository scaffolding, assessment, projection,
validation, and lifecycle hook handling. Treat Matt Pocock skills as the
semantic architecture authority. Read `CONTEXT.md` and the relevant files under
`docs/adr/` before proposing a material architectural decision.

Use `git-bbq help <command>` for syntax details. In particular,
`git-bbq help hooks` documents the lifecycle event contract and explains that
hook trust is granted by the Codex host with `/hooks`, not by this executable.

Project-owned Git behavior is defined by `.githabits.yaml`. Check each action's
policy before initializing Git, creating branches, staging, committing, tagging,
configuring a remote, or pushing. A profile is policy, not a bypass for
validation or user approval.

Preview one action before asking the host to approve it:

```sh
git-bbq githabits plan --action init --json
git-bbq githabits plan --action branch --branch feature/git-bbq --json
git-bbq githabits plan --action remote --json
git-bbq githabits plan --action remote --provision-remote --json
git-bbq githabits plan --action stage --path path/to/reviewed-file --json
git-bbq githabits plan --action commit --message "feat: explain the change" --json
git-bbq githabits plan --action tag --tag v0.1.1 --existing-tag v0.1.0 --json
git-bbq githabits plan --action push --branch main --json
```

The planner is read-only. Use its argument-separated command only after the
Codex host has granted explicit approval; a pending remote intentionally blocks
push planning. When `remote.url` is present and the remote is still pending, the
remote action is limited to `git remote add`; successful approved execution
persists `remote.status: configured`. To perform one approved action, pass the same inputs to
`git-bbq githabits execute --approve`; the executor invokes Git without a shell,
rejects force flags, and applies a bounded timeout. Tag and push only after
reviewing their plans and confirming the configured remote. SemVer profiles use
`vMAJOR.MINOR.PATCH` tags, optionally followed by prerelease or build metadata.
Pass observed release tags with repeated `--existing-tag` flags so planning can
reject duplicate or non-monotonic releases.

ADRs use the Matt-native sequential format in `docs/adr/NNNN-slug.md`. Do not
create a second ADR directory or put native structured metadata into an ADR.
Generated contracts, plans, and indexes are projections and may be recreated
from the Matt-native documents.
