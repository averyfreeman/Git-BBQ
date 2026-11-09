# Git BBQ

Git BBQ is a Go-first scaffolding and repository-lifecycle tool for coding
agents. It gives a project a place for its vocabulary, architectural decisions,
language skills, lifecycle hooks, generated projections, and Git policy.

Git BBQ does not call a model. The host agent still does the architecture work;
Git BBQ keeps the result in the repository and checks that the pieces agree.

## Quick start: Secure-Web-Printer

Here is a deliberately practical example. Imagine you are starting
Secure-Web-Printer, a Go HTTPS server with an embedded Let's Encrypt ACME
requester, HTMX and Markdown templates, and release builds for macOS, Linux,
and Windows on both x86_64 and ARM64.

Compile Git BBQ first, then use the resulting binary for the project workflow:

~~~sh
go build -o ./git-bbq ./cmd/git-bbq

./git-bbq init \
  --name Secure-Web-Printer \
  --problem "Build Secure-Web-Printer, a Go HTTPS server with an embedded Let's Encrypt ACME requester, HTMX and Markdown templates, and release binaries for aarch64-darwin, gnu-linux-x86_64, gnu-linux-aarch64, Windows x86_64, and Windows aarch64." \
  --language go \
  ./Secure-Web-Printer

./git-bbq adr new \
  --title "Set Secure-Web-Printer release targets" \
  --context "Secure-Web-Printer needs one release matrix for its Go HTTPS server across macOS arm64, Linux x86_64 and arm64, and Windows x86_64 and arm64." \
  --decision "Use darwin/arm64 (aarch64-darwin), linux/amd64 (gnu-linux-x86_64), linux/arm64 (gnu-linux-aarch64), windows/amd64 (Windows x86_64), and windows/arm64 (Windows aarch64)." \
  --why "The names make the release promise readable while Go's GOOS and GOARCH pairs keep builds reproducible." \
  ./Secure-Web-Printer

./git-bbq project ./Secure-Web-Printer
./git-bbq validate ./Secure-Web-Printer
~~~

This example records the release targets; it does not pretend that Git BBQ has
a target flag or that it writes the HTTPS server for you. The application code,
Go module, ACME integration, templates, and release pipeline still belong to
Secure-Web-Printer.

### What the example creates

init creates the project contract and agent-facing files. adr new adds the
first decision. project creates the deterministic projections, and validate
checks the result without adding another decision history.

After the full sequence above, the project contains:

~~~text
Secure-Web-Printer/
├── .agents/
│   ├── mattpocock/DEPENDENCY.yaml
│   └── skills/
│       ├── githabits/SKILL.md
│       └── go/SKILL.md
├── .gitbbq/
│   ├── .gitignore
│   ├── hooks.json
│   ├── ownership.json
│   └── session.json
├── .gitbbq-manifest.yaml
├── .githabits.yaml
├── AGENTS.md
├── CLAUDE.md
├── CONTEXT-MAP.md
├── CONTEXT.md
├── architecture-contract.yaml
├── docs/
│   └── adr/
│       ├── 0001-set-secure-web-printer-release-targets.md
│       └── index.json
└── implementation-plan.md
~~~

The useful distinction is between decisions and projections:

- CONTEXT.md and CONTEXT-MAP.md hold the project's vocabulary and context
  routing.
- AGENTS.md and CLAUDE.md point host agents at those project rules.
- .gitbbq-manifest.yaml records the problem, language profile, pinned Matt
  dependency, and required hooks.
- .githabits.yaml makes Git behavior explicit instead of leaving it to an
  agent's assumptions.
- docs/adr/0001-...md is the human-readable architectural decision.
- architecture-contract.yaml, implementation-plan.md, and docs/adr/index.json
  are generated views of the source documents.
- .gitbbq/ownership.json, .gitbbq/hooks.json, .gitbbq/session.json, and
  .gitbbq/.gitignore track Git BBQ's generated operational state.

### The five release targets

The readable product names map to Go's GOOS and GOARCH values like this:

| Release label | GOOS | GOARCH |
| --- | --- | --- |
| aarch64-darwin | darwin | arm64 |
| gnu-linux-x86_64 | linux | amd64 |
| gnu-linux-aarch64 | linux | arm64 |
| Windows x86_64 | windows | amd64 |
| Windows aarch64 | windows | arm64 |

Once the project has a go.mod and an application entry point, its release
commands can be as plain as:

~~~sh
mkdir -p dist
GOOS=darwin GOARCH=arm64 go build -o dist/Secure-Web-Printer-aarch64-darwin ./...
GOOS=linux GOARCH=amd64 go build -o dist/Secure-Web-Printer-gnu-linux-x86_64 ./...
GOOS=linux GOARCH=arm64 go build -o dist/Secure-Web-Printer-gnu-linux-aarch64 ./...
GOOS=windows GOARCH=amd64 go build -o dist/Secure-Web-Printer-windows-x86_64.exe ./...
GOOS=windows GOARCH=arm64 go build -o dist/Secure-Web-Printer-windows-aarch64.exe ./...
~~~

Those build commands are application work. Git BBQ's job is to make the
decision and its surrounding project context visible before that work begins.

## Project contract

A Git BBQ project uses a small set of predictable files:

- CONTEXT.md and CONTEXT-MAP.md for project vocabulary;
- docs/adr/ as the architectural decision source of truth;
- .gitbbq-manifest.yaml for the problem, languages, pinned Matt dependency,
  and hooks;
- .githabits.yaml for explicit Git lifecycle policy;
- .agents/skills/ for githabits and selected language skills;
- generated architecture-contract.yaml, implementation-plan.md, and
  docs/adr/index.json projections.

The detailed contract is in docs/GIT_BBQ.md.

## Existing repositories

If you are bringing Git BBQ to an existing repository, start with
git-bbq assess. It is read-only and reports the paths Git BBQ would use plus
any conflicts it finds. git-bbq apply --approve is the separate persistence
step. Existing files stay put unless you explicitly pass --force.

For a legacy ai-architect project, run git-bbq migrate [path] --json first.
The default is read-only: it inventories the old artifacts, shows the proposed
Matt-native targets, and makes the approval boundary visible. The approved
migration is additive, keeps .ai-architect/, converts validated ADRs under
docs/adr/, and archives an incompatible legacy .githabits.yaml before
replacing it. --force is required for that legacy-config replacement.

Every new scaffold records hashes in .gitbbq/ownership.json. Run
git-bbq uninstall [path] --json to see what could be removed. Only
git-bbq uninstall [path] --approve removes unchanged files; modified,
symlinked, non-regular, conflicting, or unowned paths are left alone.

Language selection is explicit. Supported profiles are Go, Python, TypeScript,
JavaScript, Rust, Java, and C#. Git profiles are explicit too: manual and
guided authorize no Git mutations by default, while autonomous enables the
individual action switches until a project changes them.

CLI flags take precedence over project configuration, which takes precedence
over the optional user-level preference file. Interactive setup asks whether
the selected profile and languages should be remembered. The --remember flag
makes that choice explicit in non-interactive use.

## Git operations

The Git mutation seam has a read-only planning step and an explicit execution
step:

~~~sh
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
~~~

Plans render argument-separated Git commands and report whether
.githabits.yaml authorizes the action. They never execute Git, and every plan
still needs host-level approval. execute --approve revalidates the plan,
invokes Git without a shell, applies a bounded timeout, and rejects tampered
commands and force flags.

Remote planning reads remote.alias and remote.url from a pending
.githabits.yaml configuration and emits only git remote add. After that
approved command succeeds, Git BBQ records remote.status: configured; a
failed command leaves it pending. Push planning requires an explicitly
configured remote. Staging rejects absolute and parent-traversal paths, and
conventional-commit profiles reject messages without a recognized commit type.
SemVer profiles require vMAJOR.MINOR.PATCH, with optional prerelease and build
metadata. Observed release tags can be supplied with repeated
--existing-tag flags so planning can reject duplicate or older releases.
--provision-remote is a separate approval-gated GitHub CLI plan using
gh repo create --source . --remote <alias>; it never includes --push.

## Codex plugin

The Codex package is built from the same Go CLI:

~~~sh
python3 scripts/build_git_bbq_plugin.py \
  --output .tmp/git-bbq-plugin \
  --target x86_64-linux \
  --force
~~~

The package embeds the git-bbq runtime and exposes the structured Git BBQ
identity plus the direct $git-bbq skill. It installs five lifecycle hooks:
UserPromptSubmit, PreToolUse, PostToolUse, PostCompact, and Stop. The runtime
accepts one JSON event on standard input and fails closed when the workspace
does not have a valid Git BBQ manifest or hook configuration.

## Development

~~~sh
gofmt -w cmd internal
go test ./...
go vet ./...
go build ./cmd/git-bbq
go run ./cmd/git-bbq schema .
~~~

Generated schemas under schemas/gitbbq/ come from the Go contracts and should
not be hand-edited.

## Safety boundaries

Git BBQ treats repository contents as untrusted data, keeps static inspection
bounded, preserves existing files by default, and separates read-only plans
from approved Git mutations. It never force-pushes, rewrites shared history,
silently creates a remote, or treats a generated projection as a second
decision history.
