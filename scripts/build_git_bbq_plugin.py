#!/usr/bin/env python3
"""Build the standalone Git BBQ Codex plugin from the Go runtime."""

from __future__ import annotations

import argparse
import json
import os
import platform
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_VERSION = "0.1.0"
MATT_REPOSITORY = "https://github.com/mattpocock/skills.git"
MATT_COMMIT = "c55ee460"
TARGETS = {
    "windows-x86_64": ("windows", "amd64", "git-bbq.exe"),
    "aarch64-darwin": ("darwin", "arm64", "git-bbq"),
    "x86_64-darwin": ("darwin", "amd64", "git-bbq"),
    "x86_64-linux": ("linux", "amd64", "git-bbq"),
}


def current_target() -> str:
    system = platform.system().lower()
    machine = platform.machine().lower()
    if system == "windows":
        return "windows-x86_64"
    if system == "darwin":
        return "aarch64-darwin" if machine in {"arm64", "aarch64"} else "x86_64-darwin"
    return "x86_64-linux"


def copy_file(source: Path, destination: Path) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, destination)


def copy_matt_skills(output: Path) -> None:
    local_sources = (ROOT / ".agents/skills", ROOT / ".agents/mattpocock/skills")
    source = next((candidate for candidate in local_sources if candidate.is_dir()), None)
    if source is not None:
        shutil.copytree(source, output / "skills/mattpocock")
        return

    with tempfile.TemporaryDirectory(prefix="git-bbq-matt-") as temporary:
        checkout = Path(temporary) / "skills"
        subprocess.run(  # noqa: S603 - fixed upstream repository and arguments
            [  # noqa: S607
                "git",
                "clone",
                "--filter=blob:none",
                "--no-checkout",
                MATT_REPOSITORY,
                str(checkout),
            ],
            cwd=ROOT,
            check=True,
        )
        subprocess.run(  # noqa: S603 - fixed checkout and pinned commit
            ["git", "-C", str(checkout), "checkout", "--detach", MATT_COMMIT],  # noqa: S607
            cwd=ROOT,
            check=True,
        )
        source = checkout / "skills"
        if not source.is_dir():
            raise SystemExit(f"pinned Matt checkout has no skills directory: {source}")
        shutil.copytree(source, output / "skills/mattpocock")


def build(args: argparse.Namespace) -> Path:
    target = args.target or current_target()
    try:
        goos, goarch, executable = TARGETS[target]
    except KeyError as exc:
        choices = ", ".join(sorted(TARGETS))
        raise SystemExit(f"unsupported target {target!r}; choose from {choices}") from exc

    output = (ROOT / args.output).resolve()
    if output.exists():
        if not args.force:
            raise SystemExit(
                f"output already exists; pass --force to replace generated path: {output}"
            )
        shutil.rmtree(output)

    manifest_path = ROOT / "adapters/codex/templates/git-bbq-plugin.json"
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    manifest["version"] = args.plugin_version
    plugin_manifest = output / ".codex-plugin/plugin.json"
    plugin_manifest.parent.mkdir(parents=True, exist_ok=True)
    plugin_manifest.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    copy_file(ROOT / "adapters/codex/templates/git-bbq-hooks.json", output / "hooks/hooks.json")
    copy_file(ROOT / "adapters/codex/templates/git-bbq-openai.yaml", output / "openai.yaml")
    copy_file(
        ROOT / "adapters/codex/templates/git-bbq-skill.md",
        output / "skills/git-bbq/SKILL.md",
    )

    copy_matt_skills(output)

    runtime = output / "runtime/git-bbq"
    runtime.mkdir(parents=True, exist_ok=True)
    runtime_path = runtime / executable
    environment = os.environ.copy()
    environment.update({"GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": "0"})
    command = [args.go, "build", "-trimpath", "-o", str(runtime_path), "./cmd/git-bbq"]
    subprocess.run(command, cwd=ROOT, env=environment, check=True)  # noqa: S603
    runtime_path.chmod(0o755)

    marketplace_path = ROOT / "adapters/codex/templates/git-bbq-marketplace.json"
    marketplace = json.loads(marketplace_path.read_text(encoding="utf-8"))
    (output / "marketplace.json").write_text(
        json.dumps(marketplace, indent=2) + "\n", encoding="utf-8"
    )

    required_events = {"UserPromptSubmit", "PreToolUse", "PostToolUse", "PostCompact", "Stop"}
    hook_config = json.loads((output / "hooks/hooks.json").read_text(encoding="utf-8"))
    if set(hook_config.get("hooks", {})) != required_events:
        raise SystemExit("Git BBQ plugin must package exactly the five required lifecycle hooks")
    if not runtime_path.is_file():
        raise SystemExit(f"Go runtime was not built: {runtime_path}")
    return output


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", default="dist/codex/git-bbq")
    parser.add_argument("--plugin-version", default=DEFAULT_VERSION)
    parser.add_argument("--target", choices=sorted(TARGETS))
    parser.add_argument("--go", default="go")
    parser.add_argument("--force", action="store_true")
    args = parser.parse_args()
    output = build(args)
    print(f"Git BBQ plugin built: {output}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
