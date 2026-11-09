# Git BBQ development

## Repository boundaries

- `cmd/git-bbq` is the public CLI.
- `internal/gitbbq` owns scaffolding, ADRs, projections, validation, hooks,
  githabits planning, and approved execution.
- `adapters/codex/templates` contains the portable plugin and Codex
  compatibility packaging contract.
- `scripts/build_git_bbq_plugin.py` builds the dependency-free runtime package.
- `scripts/validate_git_bbq_plugin.py` validates the generated package,
  curated skill layout, hooks, runtimes, assets, branding, and language.

Git BBQ is self-contained. New behavior belongs in this repository and must
not depend on an unrelated source tree.

## Checks

```sh
test -z "$(gofmt -l cmd internal)"
go test ./...
go vet ./...
go build ./cmd/git-bbq
go run ./cmd/git-bbq schema .
python3 scripts/build_git_bbq_plugin.py --output .tmp/git-bbq-plugin --target x86_64-linux --force
python3 scripts/validate_git_bbq_plugin.py .tmp/git-bbq-plugin --target x86_64-linux
```

## Skill packaging workflow

Package changes follow this order:

1. Resolve the exact Matt skills commit recorded in the build script.
2. Copy only the curated ADR, architecture, code, and documentation skills.
3. Remove provider-specific features and rewrite retained references for Codex.
4. Normalize copied text to US English.
5. Build and validate the Codex plugin package.

The package build accepts a local Matt checkout only when its Git `HEAD` exactly
matches the pinned commit; otherwise it checks out the pinned upstream commit.
The optional `lavish-axi` CLI may be used to review HTML architecture or
prototype artifacts, but it is not a package dependency.

For plugin packaging:

```sh
python3 scripts/build_git_bbq_plugin.py \
  --output .tmp/git-bbq-plugin \
  --target x86_64-linux \
  --force
python3 scripts/validate_git_bbq_plugin.py .tmp/git-bbq-plugin --target x86_64-linux
```

Inspect the generated package for the portable manifest, compatibility
manifest, five required hooks, direct skill directories, launchers, and
embedded runtime before using a release tag.
