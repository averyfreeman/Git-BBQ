# Git BBQ development

## Repository boundaries

- `cmd/git-bbq` is the public CLI.
- `internal/gitbbq` owns scaffolding, ADRs, projections, validation, hooks,
  githabits planning, and approved execution.
- `adapters/codex/templates` contains the portable plugin and Codex
  compatibility packaging contract.
- `scripts/build_git_bbq_plugin.py` builds local and public-oriented runtime
  packages from shared sources.
- `scripts/validate_git_bbq_plugin.py` validates the generated package,
  curated skill layout, hooks, runtimes, assets, branding, and language.

The plugin packages selected skills from its pinned upstream Matt Pocock
submodule. Package selection and adapter code live here; upstream skill
content does not.

## Checks

```sh
test -z "$(gofmt -l cmd internal)"
go test ./...
go vet ./...
go build ./cmd/git-bbq
go run ./cmd/git-bbq schema .
python3 scripts/build_git_bbq_plugin.py --output .tmp/git-bbq-plugin --variant local --target x86_64-linux --force
python3 scripts/validate_git_bbq_plugin.py .tmp/git-bbq-plugin --variant local --target x86_64-linux
python3 scripts/build_git_bbq_plugin.py --output .tmp/git-bbq-public --variant public --target x86_64-linux --force
python3 scripts/validate_git_bbq_plugin.py .tmp/git-bbq-public --variant public --target x86_64-linux
PYTHONPATH=scripts python3 -m unittest scripts/test_build_git_bbq_plugin.py
```

## Skill packaging workflow

Package changes follow this order:

1. Read the pinned upstream repository and commit from
   `git-bbq-curation.json`; verify `.agents/skills` points to that clean
   upstream checkout.
2. Require the curation selection to match every skill in the stable
   `engineering` and `productivity` categories at that commit.
3. Export selected skill directories from the pinned Git tree and package them
   under their upstream directory names.
4. Preserve upstream instructions and support files. The `SKILL.md` metadata
   adapter maps `disable-model-invocation: true` to adjacent Codex policy
   `allow_implicit_invocation: false`, and carries `argument-hint` into
   namespaced standard metadata.
5. Build both plugin manifests, declare the bundled setup skill as OpenAI
   onboarding in both manifests, and validate Agent Skills fields, discovery
   paths, upstream names, variant-specific hooks, runtimes, and OpenAI metadata
   constraints.

The package build accepts the `.agents/skills` checkout only when its Git
`HEAD`, origin, and clean working tree match the upstream pin. It exports the
selected files from that Git commit, so local ignored files cannot enter the
package. If the submodule is absent, the builder temporarily clones the same
upstream repository at the exact pin. The generated
`skills/matt-skills-manifest.json` records each packaged source path, upstream
name, repository, and commit. Update the curation manifest and submodule pin
together when adopting another upstream revision.
The optional `lavish-axi` CLI may be used to review HTML architecture or
prototype artifacts, but it is not a package dependency.

For plugin packaging:

```sh
python3 scripts/build_git_bbq_plugin.py \
  --output .tmp/git-bbq-plugin \
  --variant local \
  --target x86_64-linux \
  --force
python3 scripts/validate_git_bbq_plugin.py .tmp/git-bbq-plugin --variant local --target x86_64-linux
```

Inspect both generated packages for portable and compatibility manifests,
direct skill directories, launchers, and embedded runtimes. The local variant
has exactly five required hooks; the public variant has no hook directory or
manifest hook reference.
