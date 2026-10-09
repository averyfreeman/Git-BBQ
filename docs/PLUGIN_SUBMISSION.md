# Git BBQ public plugin submission

This file is the release handoff for the OpenAI plugin submission portal. It
records the package contract and test evidence without claiming public
publication.

## Listing metadata

- Plugin ID: git-bbq
- Display name: Git BBQ
- Version: 0.4.1
- Category: Developer Tools
- Repository: https://github.com/averyfreeman/git-bbq
- Capabilities: Read and Write
- Submission artifact: public-oriented, hook-free skills package with a bundled
  local Go CLI; no MCP server
- Runtime targets: aarch64-darwin, x86_64-darwin, x86_64-linux, and windows-x86_64

Git BBQ's local Codex package scaffolds a repository, records durable project
context and ADRs, projects validated architecture documents, and plans guarded
Git operations. The separate local variant includes five deterministic hooks
that fail closed when the workspace does not contain a valid Git BBQ project
contract.

The packaged skill set contains all stable skills from the upstream
`engineering` and `productivity` categories recorded in the generated Matt
skills manifest. Skills retain their upstream names and source text. The build
adapts the non-standard `disable-model-invocation` field into Codex's adjacent
invocation policy so packaged `SKILL.md` frontmatter follows Agent Skills.

Build the public-oriented artifact with `--variant public`; it omits the hook
directory, manifest hook setting, and hook-specific instructions from Git
BBQ's own skill. The local variant remains available for Codex desktop users
who want lifecycle hooks. OpenAI's current package guidance says lifecycle
hooks are for manually installed Codex desktop plugins and packages that
contain them are ineligible for the public plugin directory. Web installation
does not deploy hook scripts. A hook-free package clears that specific
eligibility constraint, but does not guarantee listing approval; skills-only
plugins may have additional review requirements. See [OpenAI package
guidance](https://developers.openai.com/plugins/build/plugins) and [submission
requirements](https://developers.openai.com/plugins/deploy/submission).

The public-oriented artifact still includes the Go CLI binaries for Codex
execution environments that can run local binaries. ChatGPT web installs do
not gain access to the user's local repository or a way to execute that CLI.
Hosted repository operations would need a remote service such as MCP, which is
outside this implementation.

The portable and Codex compatibility manifests point to the bundled upstream
`setup-matt-pocock-skills` skill as the plugin onboarding skill. The package
validator checks that both declarations match and that the skill is present.

## Starter prompts

1. Scaffold this repository for agent-driven development.
2. Create or review durable architecture records and project context.
3. Review the repository's Git workflow policy before proposing a mutation.

## Positive tests

1. Install the local variant in an isolated Codex CLI home and verify git-bbq
   version and upstream skill discovery.
2. Validate the public-oriented artifact: it contains the same curated skills
   and CLI targets but no hook files, hook-specific skill guidance, or manifest
   hook references.
3. Run `git-bbq help hooks` in the local variant and verify event syntax plus
   `/hooks` trust guidance.
4. Install the local plugin in a fresh Codex profile and verify the setup skill is
   offered as onboarding; complete setup in a fixture repository and confirm
   its tracker and domain-document choices are preserved.
5. Start a new thread in Codex and invoke a Git BBQ skill.
6. Run git-bbq assess against an existing repository and confirm it is
   read-only.
7. Run git-bbq project after creating an ADR and verify the projections.
8. Send valid JSON to each of the five local-variant hooks and verify
   structured responses.

## Negative tests

1. Send malformed hook JSON and confirm the hook denies the event safely.
2. Request a Git mutation without an approval flag and confirm no Git command
   runs.
3. Run the hook in a directory without a valid manifest and confirm it fails
   closed without creating files.

## Release gates

- [ ] Both local and public-oriented package validators pass for every target
  artifact; public checks confirm no hook configuration or references remain.
- [ ] The official plugin validator passes against the compatibility manifest.
- [ ] The package contains exactly the stable upstream selection.
- [ ] Skill names, metadata, and immediate-child discovery paths pass the Agent
  Skills and Agent Plugins checks.
- [ ] Legal URLs resolve to the repository privacy and terms pages.
- [ ] The public-oriented ZIP passes portal validation and bundled-skill safety
  scans; publisher identity and required attestations are ready in the portal.
- [ ] Any local-runtime or filesystem-access review requested by OpenAI is
  complete.
- [ ] OpenAI review is complete before selecting public publication.
