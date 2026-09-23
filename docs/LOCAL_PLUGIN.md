# Local Codex plugin installation

Build a local multi-platform package from the repository root:

~~~sh
python3 scripts/build_git_bbq_plugin.py \
  --output plugins/git-bbq \
  --target all \
  --force
python3 scripts/validate_git_bbq_plugin.py plugins/git-bbq
~~~

Register and install the repository marketplace:

~~~sh
codex plugin marketplace add .
codex plugin add git-bbq@git-bbq-local
codex plugin list
~~~

The repository marketplace is `.agents/plugins/marketplace.json`, and
`.codex/config.toml` enables Git BBQ for this trusted repository. The generated
`plugins/git-bbq` directory is intentionally ignored because it contains
compiled target runtimes.

Restart the Codex host after changing the package, then test in a new thread.

## Lifecycle hook trust

There is no separate per-plugin enable switch in the plugin settings page. The
package declares its lifecycle hooks, and the host loads them when its hooks
feature is enabled. This repository makes that intent explicit in
`.codex/config.toml`.

The plugin settings page shows package metadata; it is not the lifecycle-hook
trust control. In Codex CLI, open the hook review surface with:

~~~text
/hooks
~~~

Review and trust the current Git BBQ hook definition there. Trust is tied to the
definition currently installed by the host. If `hooks/hooks.json` or a hook
command changes, the host requires another review and may skip the hook until
it is trusted again.

If `/hooks` does not show Git BBQ after restarting, check that the host has not
set `[features] hooks = false`; a host or administrator policy can still
override project configuration.

`git-bbq help hooks` explains the runtime event syntax. The `git-bbq` command
can execute a hook event, but it cannot grant host-level trust. Use
`--dangerously-bypass-hook-trust` only for disposable local diagnostics, never
as the normal installation path.

Git BBQ uses the skills-only marketplace shape with local Codex lifecycle hooks;
it does not require an MCP server. Public submission remains separate from this
local marketplace installation; see `PLUGIN_SUBMISSION.md` for release gates.
