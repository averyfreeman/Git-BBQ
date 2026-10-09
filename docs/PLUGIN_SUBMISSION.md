# Git BBQ public plugin submission

This file is the release handoff for the OpenAI plugin submission portal. It
records the package contract and test evidence without claiming public
publication.

## Listing metadata

- Plugin ID: git-bbq
- Display name: Git BBQ
- Version: 0.4.0
- Category: Developer Tools
- Repository: https://github.com/averyfreeman/git-bbq
- Capabilities: Read and Write
- Package type: skills-only plugin plus OpenAI lifecycle hooks; no MCP server
- Runtime targets: aarch64-darwin, x86_64-darwin, x86_64-linux, and windows-x86_64

Git BBQ scaffolds a repository, records durable project context and ADRs,
projects validated architecture documents, and plans guarded Git operations.
The bundled hooks are deterministic and fail closed when the workspace does
not contain a valid Git BBQ project contract.

The packaged skill set contains all stable skills from the upstream
`engineering` and `productivity` categories recorded in the generated Matt
skills manifest. Skills retain their upstream names and source text. The build
adapts the non-standard `disable-model-invocation` field into Codex's adjacent
invocation policy so packaged `SKILL.md` frontmatter follows Agent Skills.

The package contains lifecycle hooks for local Codex use. OpenAI's current
plugin guidance says plugins containing lifecycle hooks are not eligible for
the public plugin directory, so this package is not ready for public
submission while those hooks remain. Public submission is outside the current
release scope.

The portable and Codex compatibility manifests point to the bundled upstream
`setup-matt-pocock-skills` skill as the plugin onboarding skill. The package
validator checks that both declarations match and that the skill is present.

## Starter prompts

1. Scaffold this repository for agent-driven development.
2. Create or review durable architecture records and project context.
3. Review the repository's Git workflow policy before proposing a mutation.

## Positive tests

1. Install the local package in an isolated Codex CLI home and verify git-bbq
   version and upstream skill discovery.
2. Run `git-bbq help hooks` and verify event syntax plus `/hooks` trust guidance.
3. Install the plugin in a fresh Codex profile and verify the setup skill is
   offered as onboarding; complete setup in a fixture repository and confirm
   its tracker and domain-document choices are preserved.
4. Start a new thread in Codex and invoke a Git BBQ skill.
5. Run git-bbq assess against an existing repository and confirm it is
   read-only.
6. Run git-bbq project after creating an ADR and verify the projections.
7. Send valid JSON to each of the five hooks and verify structured responses.

## Negative tests

1. Send malformed hook JSON and confirm the hook denies the event safely.
2. Request a Git mutation without an approval flag and confirm no Git command
   runs.
3. Run the hook in a directory without a valid manifest and confirm it fails
   closed without creating files.

## Release gates

- [ ] The package validator passes for every target artifact.
- [ ] The official plugin validator passes against the compatibility manifest.
- [ ] The package contains exactly the stable upstream selection.
- [ ] Skill names, metadata, and immediate-child discovery paths pass the Agent
  Skills and Agent Plugins checks.
- [ ] Legal URLs resolve to the repository privacy and terms pages.
- [ ] Verified publisher identity and selected release regions are ready in the
  submission portal.
- [ ] Any local-runtime or filesystem-access review requested by OpenAI is
  complete.
- [ ] OpenAI review is complete before selecting public publication.
