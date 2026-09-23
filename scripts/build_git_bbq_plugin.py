#!/usr/bin/env python3
"""Build the standalone Git BBQ Codex plugin from the Go runtime."""

from __future__ import annotations

import argparse
import json
import os
import platform
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_VERSION = "0.2.0"
MATT_REPOSITORY = "https://github.com/mattpocock/skills.git"
MATT_COMMIT = "c55ee46073ed923f86ce59a5eb3b6d895095d1b7"
MATT_SKILL_PATHS = (
    "engineering/ask-matt",
    "engineering/code-review",
    "engineering/codebase-design",
    "engineering/diagnosing-bugs",
    "engineering/domain-modeling",
    "engineering/grill-with-docs",
    "engineering/implement",
    "engineering/improve-codebase-architecture",
    "engineering/prototype",
    "engineering/research",
    "engineering/resolving-merge-conflicts",
    "engineering/tdd",
    "productivity/grilling",
    "productivity/writing-for-agents",
)
US_ENGLISH_REPLACEMENTS = {
    "behaviour": "behavior",
    "behaviours": "behaviors",
    "colour": "color",
    "colours": "colors",
    "centre": "center",
    "centres": "centers",
    "labelled": "labeled",
    "labelling": "labeling",
    "modelling": "modeling",
    "optimise": "optimize",
    "optimised": "optimized",
    "optimising": "optimizing",
    "prioritise": "prioritize",
    "prioritised": "prioritized",
    "prioritising": "prioritizing",
    "recognised": "recognized",
    "summarise": "summarize",
    "summarised": "summarized",
    "summarising": "summarizing",
    "travelling": "traveling",
    "artefact": "artifact",
    "artefacts": "artifacts",
    "authorise": "authorize",
    "authorised": "authorized",
    "authorising": "authorizing",
    "minimise": "minimize",
    "minimised": "minimized",
    "minimising": "minimizing",
    "organisation": "organization",
    "organisations": "organizations",
    "programme": "program",
    "programmes": "programs",
    "favour": "favor",
    "licence": "license",
}
TEXT_SUFFIXES = {
    ".cjs",
    ".go",
    ".html",
    ".js",
    ".json",
    ".md",
    ".mjs",
    ".py",
    ".sh",
    ".txt",
    ".ts",
    ".yaml",
    ".yml",
}
CLAUDE_FRONTMATTER = re.compile(r"(?m)^disable-model-invocation:\s*(?:true|false)\s*\n")
BRANDING_REPLACEMENTS = (
    ("Claude Code", "Codex CLI"),
    ("Claude → Codex", "Codex"),
    ("CLAUDE.md", "AGENTS.md"),
    ("CLAUDE_PROJECT_DIR", "CODEX_PROJECT_DIR"),
    (".claude/", ".codex/"),
    ("Claude", "Codex"),
    ("claude", "codex"),
    ("Anthropic", "OpenAI"),
    ("anthropic", "openai"),
)
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


def git_head(path: Path) -> str | None:
    try:
        result = subprocess.run(
            ["git", "-C", str(path), "rev-parse", "HEAD"],  # noqa: S603, S607
            check=True,
            capture_output=True,
            text=True,
        )
    except (OSError, subprocess.CalledProcessError):
        return None
    return result.stdout.strip()


def local_pinned_matt_source() -> Path | None:
    local_sources = (ROOT / ".agents/skills", ROOT / ".agents/mattpocock/skills")
    for candidate in local_sources:
        if candidate.is_dir() and git_head(candidate) == MATT_COMMIT:
            return candidate
    return None


def copy_matt_skills(output: Path) -> None:
    source = local_pinned_matt_source()
    if source is not None:
        copy_matt_skill_directories(source, output)
        return
    with tempfile.TemporaryDirectory(prefix="git-bbq-matt-") as temporary:
        checkout = Path(temporary) / "skills"
        subprocess.run(  # noqa: S603 - fixed upstream repository and arguments
            [
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
        if git_head(checkout) != MATT_COMMIT:
            raise SystemExit(f"Matt checkout is not pinned to {MATT_COMMIT}")
        source = checkout / "skills"
        if not source.is_dir():
            raise SystemExit(f"pinned Matt checkout has no skills directory: {source}")
        copy_matt_skill_directories(source, output)


def copy_matt_skill_directories(source: Path, output: Path) -> None:
    destination_root = output / "skills"
    destination_root.mkdir(parents=True, exist_ok=True)
    for relative_path in MATT_SKILL_PATHS:
        skill = source / relative_path
        if not (skill / "SKILL.md").is_file():
            raise SystemExit(f"pinned Matt skill is missing SKILL.md: {relative_path}")
        relative_name = "-".join(Path(relative_path).parts)
        destination = destination_root / f"mattpocock-{relative_name}"
        shutil.copytree(skill, destination)
        sanitize_skill_directory(destination)
        rewrite_curated_skill(relative_path, destination)


def preserve_case(replacement: str, value: str) -> str:
    if value[:1].isupper():
        return replacement.capitalize()
    return replacement


def normalize_us_english(content: str) -> str:
    for british, american in US_ENGLISH_REPLACEMENTS.items():
        pattern = re.compile(rf"\b{re.escape(british)}\b", re.IGNORECASE)
        content = pattern.sub(lambda match: preserve_case(american, match.group(0)), content)
    return content


def sanitize_text(content: str) -> str:
    content = CLAUDE_FRONTMATTER.sub("", content)
    for source, replacement in BRANDING_REPLACEMENTS:
        content = content.replace(source, replacement)
    content = content.replace("disable-model-invocation: true", "policy.allow_implicit_invocation: false")
    content = content.replace("disable-model-invocation", "policy.allow_implicit_invocation")
    return normalize_us_english(content)


def sanitize_skill_directory(skill_directory: Path) -> None:
    for path in skill_directory.rglob("*"):
        if not path.is_file() or path.suffix.lower() not in TEXT_SUFFIXES:
            continue
        try:
            content = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        sanitized = sanitize_text(content)
        if sanitized != content:
            path.write_text(sanitized, encoding="utf-8")


def rewrite_text_file(path: Path, replacements: tuple[tuple[str, str], ...]) -> None:
    content = path.read_text(encoding="utf-8")
    for source, replacement in replacements:
        content = content.replace(source, replacement)
    path.write_text(content, encoding="utf-8")


def rewrite_curated_skill(relative_path: str, destination: Path) -> None:
    if relative_path == "engineering/ask-matt":
        template_root = ROOT / "adapters/codex/templates/matt/ask-matt"
        copy_file(template_root / "SKILL.md", destination / "SKILL.md")
        copy_file(template_root / "PHASE-BOUNDARIES.md", destination / "PHASE-BOUNDARIES.md")
        return

    if relative_path == "engineering/code-review":
        rewrite_text_file(
            destination / "SKILL.md",
            (
                (
                    "The issue tracker should have been provided to you. If `docs/agents/issue-tracker.md` is missing, tell the user to run `/setup-matt-pocock-skills`.",
                    "Use the spec source available in the repository or supplied by the user. If no spec is available, report that limitation and keep the Spec axis separate from Standards.",
                ),
                (
                    "1. Issue references in the commit messages (`#123`, `Closes #45`, GitLab `!67`, etc.), fetched via the workflow in `docs/agents/issue-tracker.md`.\n2. A path the user passed as an argument.\n3. A spec file under `docs/`, `specs/`, or `.scratch/` matching the branch name or feature.\n4. If nothing is found, ask the user where the spec is. If they say there isn't one, the **Spec** sub-agent will skip and report \"no spec available\".",
                    "1. A path the user supplied as the spec source.\n2. An ADR or spec under `docs/`, `specs/`, or `.scratch/` matching the branch or feature.\n3. A relevant locally available commit-message reference.\n4. If nothing is found, report \"no spec available\" and keep the Spec axis separate from Standards.",
                ),
            ),
        )

    if relative_path == "engineering/improve-codebase-architecture":
        rewrite_text_file(
            destination / "SKILL.md",
            (
                (
                    "Write a self-contained HTML file to the OS temp directory so nothing lands in the repo. Resolve the temp dir from `$TMPDIR`, falling back to `/tmp` (or `%TEMP%` on Windows), and write to `<tmpdir>/architecture-review-<timestamp>.html` so each run gets a fresh file. Open it for the user (`xdg-open <path>` on Linux, `open <path>` on macOS, `start <path>` on Windows) and tell them the absolute path.",
                    "Write a self-contained HTML file to the OS temp directory so nothing lands in the repo. Resolve the temp dir from `$TMPDIR`, falling back to `/tmp` or `%TEMP%`, and write to `<tmpdir>/architecture-review-<timestamp>.html`. If `lavish-axi` is available, run `lavish-axi <path>` and keep `lavish-axi poll <path>` in the foreground when the user wants an annotation loop; otherwise use the host's direct-open command. Keep local assets relative so direct-open and `lavish-axi export` remain usable.",
                ),
            ),
        )
        rewrite_text_file(
            destination / "HTML-REPORT.md",
            (
                (
                    "The architectural review is rendered as a single self-contained HTML file in the OS temp directory. Tailwind and Mermaid both come from CDNs. Mermaid handles graph-shaped diagrams reliably; hand-built divs and inline SVG handle the more editorial visuals (mass diagrams, cross-sections). Mix the two: don't lean on Mermaid for everything, it'll start to look generic.",
                    "The architectural review is rendered as a single portable HTML file in the OS temp directory. Keep local assets relative, give the page an explicit background, and preserve direct-open behavior. Use inline SVG for ordinary diagrams; use Mermaid only when the user explicitly wants an editable Lavish whiteboard. When available, `lavish-axi <path>` provides the Codex review surface and `lavish-axi export <path>` provides a portable copy.",
                ),
            ),
        )

    if relative_path == "engineering/prototype":
        rewrite_text_file(
            destination / "SKILL.md",
            (
                (
                    "A logic demo is a single HTML file the user double-clicks.",
                    "A logic demo is a single HTML file the user can open directly. When `lavish-axi` is available, use it for annotation and foreground feedback polling; direct-open remains the fallback.",
                ),
            ),
        )


def compatibility_manifest(portable: dict) -> dict:
    extension = portable["extensions"]["com.openai"]
    compatibility = {
        key: portable[key]
        for key in (
            "name",
            "version",
            "description",
            "author",
            "homepage",
            "repository",
            "license",
            "keywords",
        )
        if key in portable
    }
    compatibility["skills"] = "./skills/"
    compatibility["interface"] = extension["interface"]
    return compatibility


def write_launchers(output: Path) -> None:
    launcher = output / "runtime/git-bbq/git-bbq"
    launcher.parent.mkdir(parents=True, exist_ok=True)
    launcher.write_text(
        """#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
case "$(uname -s):$(uname -m)" in
  Darwin:arm64|Darwin:aarch64) target="aarch64-darwin" ;;
  Darwin:x86_64|Darwin:amd64) target="x86_64-darwin" ;;
  Linux:x86_64|Linux:amd64) target="x86_64-linux" ;;
  *)
    echo "unsupported Git BBQ runtime platform: $(uname -s)/$(uname -m)" >&2
    exit 1
    ;;
esac

binary="$root/bin/$target/git-bbq"
if [ ! -x "$binary" ]; then
  echo "Git BBQ runtime target is not bundled: $target" >&2
  exit 1
fi
exec "$binary" "$@"
""",
        encoding="utf-8",
    )
    launcher.chmod(0o755)

    windows_launcher = output / "runtime/git-bbq/git-bbq.cmd"
    windows_launcher.write_text(
        """@echo off
"%~dp0..\\bin\\windows-x86_64\\git-bbq.exe" %*
""",
        encoding="utf-8",
    )


def build(args: argparse.Namespace) -> Path:
    requested_target = args.target or current_target()
    targets = sorted(TARGETS) if requested_target == "all" else [requested_target]
    for target in targets:
        if target not in TARGETS:
            choices = ", ".join(["all", *sorted(TARGETS)])
            raise SystemExit(f"unsupported target {target!r}; choose from {choices}")

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
    output.mkdir(parents=True, exist_ok=True)
    (output / "plugin.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    compatibility = compatibility_manifest(manifest)
    (output / ".codex-plugin/plugin.json").parent.mkdir(parents=True, exist_ok=True)
    (output / ".codex-plugin/plugin.json").write_text(
        json.dumps(compatibility, indent=2) + "\n", encoding="utf-8"
    )
    copy_file(ROOT / "adapters/codex/templates/git-bbq-hooks.json", output / "hooks/hooks.json")
    copy_file(ROOT / "adapters/codex/templates/git-bbq-openai.yaml", output / "openai.yaml")
    copy_file(ROOT / "assets/logo.svg", output / "assets/logo.svg")
    copy_file(ROOT / "THIRD_PARTY_NOTICES.md", output / "THIRD_PARTY_NOTICES.md")
    copy_file(
        ROOT / "adapters/codex/templates/git-bbq-skill.md",
        output / "skills/git-bbq/SKILL.md",
    )

    copy_matt_skills(output)

    runtime_paths = []
    for target in targets:
        goos, goarch, executable = TARGETS[target]
        runtime_path = output / "runtime/bin" / target / executable
        runtime_path.parent.mkdir(parents=True, exist_ok=True)
        environment = os.environ.copy()
        environment.update({"GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": "0"})
        command = [args.go, "build", "-trimpath", "-o", str(runtime_path), "./cmd/git-bbq"]
        subprocess.run(command, cwd=ROOT, env=environment, check=True)  # noqa: S603
        runtime_path.chmod(0o755)
        runtime_paths.append(runtime_path)
    write_launchers(output)

    required_events = {"UserPromptSubmit", "PreToolUse", "PostToolUse", "PostCompact", "Stop"}
    hook_config = json.loads((output / "hooks/hooks.json").read_text(encoding="utf-8"))
    if set(hook_config.get("hooks", {})) != required_events:
        raise SystemExit("Git BBQ plugin must package exactly the five required lifecycle hooks")
    if not all(path.is_file() for path in runtime_paths):
        raise SystemExit("one or more Go runtime targets were not built")
    if not (output / "plugin.json").is_file():
        raise SystemExit("portable plugin manifest was not built")
    if not (output / ".codex-plugin/plugin.json").is_file():
        raise SystemExit("Codex compatibility manifest was not built")
    return output


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", default="dist/codex/git-bbq")
    parser.add_argument("--plugin-version", default=DEFAULT_VERSION)
    parser.add_argument("--target", choices=["all", *sorted(TARGETS)])
    parser.add_argument("--go", default="go")
    parser.add_argument("--force", action="store_true")
    args = parser.parse_args()
    output = build(args)
    print(f"Git BBQ plugin built: {output}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
