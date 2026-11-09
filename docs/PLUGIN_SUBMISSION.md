# Git BBQ public plugin submission

This file is the release handoff for the OpenAI plugin submission portal. It
records the package contract and test evidence without claiming public
publication.

## Listing metadata

- Plugin ID: git-bbq
- Display name: Git BBQ
- Version: 0.2.0
- Category: Developer Tools
- Repository: https://github.com/averyfreeman/git-bbq
- Capabilities: Read and Write
- Package type: skills-only plugin plus OpenAI lifecycle hooks; no MCP server
- Runtime targets: aarch64-darwin, x86_64-darwin, x86_64-linux, and windows-x86_64

Git BBQ scaffolds a repository, records durable project context and ADRs,
projects validated architecture documents, and plans guarded Git operations.
The bundled hooks are deterministic and fail closed when the workspace does
not contain a valid Git BBQ project contract.

The packaged skill set is intentionally limited to ADR creation, architecture
revision, code revision, and documentation revision. Matt skills are imported
from the pinned commit, provider-specific references are removed or rewritten
for Codex, and copied prose is normalized to US English.

## Starter prompts

1. Scaffold this repository for agent-driven development.
2. Create or review durable architecture records and project context.
3. Review the repository's Git workflow policy before proposing a mutation.

## Positive tests

1. Install the local package in Codex CLI and verify git-bbq version.
2. Run `git-bbq help hooks` and verify event syntax plus `/hooks` trust guidance.
3. Start a new thread in Codex and invoke the Git BBQ skill.
4. Run git-bbq assess against an existing repository and confirm it is
   read-only.
5. Run git-bbq project after creating an ADR and verify the projections.
6. Send valid JSON to each of the five hooks and verify structured responses.

## Negative tests

1. Send malformed hook JSON and confirm the hook denies the event safely.
2. Request a Git mutation without an approval flag and confirm no Git command
   runs.
3. Run the hook in a directory without a valid manifest and confirm it fails
   closed without creating files.

## Release gates

- [ ] The package validator passes for every target artifact.
- [ ] The official plugin validator passes against the compatibility manifest.
- [ ] The package contains only the curated skill allowlist.
- [ ] Branding and language scans find no provider-specific or non-US terms.
- [ ] Legal URLs resolve to the repository privacy and terms pages.
- [ ] Verified publisher identity and selected release regions are ready in the
  submission portal.
- [ ] Any local-runtime or filesystem-access review requested by OpenAI is
  complete.
- [ ] OpenAI review is complete before selecting public publication.
