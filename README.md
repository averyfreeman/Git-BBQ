# Git BBQ

Git BBQ is a Go-first scaffolding and repository-lifecycle tool for coding
agents. It creates durable project context, Matt-native ADRs, language skills,
Codex hooks, deterministic projections, and explicit Git workflow policy.

Git BBQ does not call a model. The host agent performs architecture reasoning;
Git BBQ owns project bootstrap, validation, projections, lifecycle hooks, and
approval-gated Git operations.

## Quick start

From a new or existing repository:

```sh
go run ./cmd/git-bbq init \
  --problem "Agents need a portable project scaffold." \
  --language go \
  --language python \
  ./my-project

go run ./cmd/git-bbq adr new \
  --title "Use Matt-native ADRs" \
  --context "Agents need durable decisions." \
  --decision "Use docs/adr files." \
  --why "The format is host-neutral." \
  ./my-project

go run ./cmd/git-bbq project ./my-project
go run ./cmd/git-bbq validate ./my-project
```

Use `git-bbq assess` before changing an existing repository. Persistence requires
the separate `git-bbq apply --approve` or migration approval command.

## Project contract

A Git BBQ project uses:

- `CONTEXT.md` and `CONTEXT-MAP.md` for project vocabulary;
- `docs/adr/` as the architectural decision source of truth;
- `.gitbbq-manifest.yaml` for project and dependency metadata;
- `.githabits.yaml` for explicit Git lifecycle policy;
- `.agents/skills/` for githabits and selected language skills;
- generated `architecture-contract.yaml`, `implementation-plan.md`, and
  `docs/adr/index.json` projections.

The detailed contract is in [docs/GIT_BBQ.md](docs/GIT_BBQ.md).

## Codex plugin

Build the standalone Codex package, including its short-lived Go runtime:

```sh
python3 scripts/build_git_bbq_plugin.py \
  --output .tmp/git-bbq-plugin \
  --target x86_64-linux \
  --force
```

The package contains exactly five lifecycle hooks and embeds the `git-bbq`
runtime. Matt Pocock skills are copied from a local checkout when available or
from the pinned upstream commit during a package build.

## Development

```sh
gofmt -w cmd internal
go test ./...
go vet ./...
go build ./cmd/git-bbq
go run ./cmd/git-bbq schema .
```

Generated schemas under `schemas/gitbbq/` are derived from Go contracts and
must not be hand-edited.

## Safety boundaries

Git BBQ treats repository contents as untrusted data, keeps static inspection
bounded, preserves existing files by default, and separates read-only plans
from approved Git mutations. It never force-pushes, rewrites shared history, or
silently creates a remote.
