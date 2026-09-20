# Git BBQ development

## Repository boundaries

- `cmd/git-bbq` is the public CLI.
- `internal/gitbbq` owns scaffolding, ADRs, migration, projections, validation,
  hooks, githabits planning, and approved execution.
- `internal/legacyarchitect` is a bounded read-only parser used only to migrate
  legacy `.ai-architect` decisions.
- `adapters/codex/templates` contains the standalone Codex packaging contract.
- `scripts/build_git_bbq_plugin.py` builds the dependency-free runtime package.

Git BBQ must remain independent of the original AI Software Architect source
tree. New behavior belongs in this repository and must not import the original
project.

## Checks

```sh
test -z "$(gofmt -l cmd internal)"
go test ./...
go vet ./...
go build ./cmd/git-bbq
go run ./cmd/git-bbq schema .
git diff --exit-code
```

For plugin packaging:

```sh
python3 scripts/build_git_bbq_plugin.py \
  --output .tmp/git-bbq-plugin \
  --target x86_64-linux \
  --force
```

Inspect the generated package for the five required hooks, the expected
manifest, and the embedded runtime before using a release tag.
