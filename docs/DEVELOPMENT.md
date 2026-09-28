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
PYTHONPATH=scripts python3 -m unittest scripts/test_build_git_bbq_plugin.py
```

## Skill packaging workflow

Package changes follow this order:

1. Resolve the exact derivative commit recorded in the build script and verify
   the checked-in `.agents/skills` submodule.
2. Parse `git-bbq-curation.json` and require all 38 source paths to be present.
3. Copy only `keep` and `alias` entries, using their manifest `publicName`.
4. Rewrite provider-specific features and retained cross-skill references for
   Codex while preserving each report or artifact contract.
5. Normalize copied text to US English, build, and validate the Codex package.

The package build accepts the `.agents/skills` checkout only when its Git `HEAD`
exactly matches the pinned derivative commit; otherwise it temporarily clones
that same derivative repository at the pin. It never falls back to the upstream
Matt repository. The generated `skills/matt-skills-manifest.json` records every
packaged source path, public name, alias, repository, and commit.
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
