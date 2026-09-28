#!/usr/bin/env python3
"""Build the standalone Git BBQ Codex plugin from the Go runtime."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
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
DEFAULT_VERSION = "0.3.0"
SKILLS_REPOSITORY = "https://github.com/averyfreeman/git-bbq-matt-skills.git"
SKILLS_COMMIT = "64fb7a440ff4a5e0b3d82680b2d73c2b93e1f2fa"
CURATION_FILENAME = "git-bbq-curation.json"
SELECTION_MANIFEST_FILENAME = "skills/matt-skills-manifest.json"
PACKAGED_STATUSES = frozenset({"keep", "alias"})
CURATION_STATUSES = frozenset({"keep", "hold", "alias", "omit"})
PUBLIC_NAME_PATTERN = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
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


@dataclass(frozen=True)
class CuratedSkill:
    """Describe one manifest entry selected for Git BBQ packaging."""

    source_path: str
    public_name: str
    alias_of: str | None


@dataclass(frozen=True)
class SkillSource:
    """Describe a pinned derivative source and its manifest-driven selection."""

    repository: str
    commit: str
    checkout: Path
    source_root: str
    selection: tuple[CuratedSkill, ...]
    transform: bool = True


SOURCE_PIN = (SKILLS_REPOSITORY, SKILLS_COMMIT)


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


def local_pinned_checkout(source_spec: SkillSource) -> Path | None:
    candidate = ROOT / ".agents/skills"
    if candidate.is_dir() and git_head(candidate) == source_spec.commit:
        return candidate
    return None


def load_pinned_source(checkout: Path, repository: str, commit: str) -> SkillSource:
    manifest_path = checkout / CURATION_FILENAME
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise SystemExit(f"could not read derivative curation manifest {manifest_path}: {exc}") from exc
    selection = validate_curation(manifest, checkout)
    derivative = manifest.get("derivative", {})
    if derivative.get("repository") != repository:
        raise SystemExit(
            "curation manifest derivative repository does not match the pinned package source: "
            f"{derivative.get('repository')!r}"
        )
    return SkillSource(
        repository=repository,
        commit=commit,
        checkout=checkout,
        source_root="skills",
        selection=tuple(selection),
    )


def copy_matt_skills(output: Path, source_spec: SkillSource | None = None) -> None:
    if isinstance(source_spec, SkillSource):
        repository = source_spec.repository
        commit = source_spec.commit
        checkout = source_spec.checkout
    else:
        repository, commit = SOURCE_PIN
        checkout = None
        candidate = ROOT / ".agents/skills"
        if candidate.is_dir() and git_head(candidate) == commit:
            checkout = candidate

    if checkout is not None:
        pinned_source = load_pinned_source(checkout, repository, commit)
        copy_skill_directories(pinned_source, output)
        return

    with tempfile.TemporaryDirectory(prefix="git-bbq-skills-") as temporary:
        checkout = Path(temporary) / "git-bbq-matt-skills"
        subprocess.run(  # noqa: S603 - fixed derivative repository and arguments
            [
                "git",
                "clone",
                "--filter=blob:none",
                "--no-checkout",
                repository,
                str(checkout),
            ],
            cwd=ROOT,
            check=True,
        )
        subprocess.run(  # noqa: S603 - fixed checkout and pinned commit
            ["git", "-C", str(checkout), "checkout", "--detach", commit],  # noqa: S607
            cwd=ROOT,
            check=True,
        )
        if git_head(checkout) != commit:
            raise SystemExit(f"derivative checkout is not pinned to {commit}")
        pinned_source = load_pinned_source(checkout, repository, commit)
        copy_skill_directories(pinned_source, output)


def copy_skill_directories(source_spec: SkillSource, output: Path) -> None:
    destination_root = output / "skills"
    destination_root.mkdir(parents=True, exist_ok=True)
    name_map = {Path(item.source_path).name: item.public_name for item in source_spec.selection}
    write_selection_manifest(output, source_spec)
    for item in source_spec.selection:
        skill = source_spec.checkout / item.source_path
        if not (skill / "SKILL.md").is_file():
            raise SystemExit(f"pinned derivative skill is missing SKILL.md: {item.source_path}")
        destination = destination_root / f"mattpocock-{item.public_name}"
        shutil.copytree(skill, destination)
        if source_spec.transform:
            sanitize_skill_directory(destination, name_map)
            rewrite_curated_skill(item.source_path, destination, item.public_name)


def validate_curation(manifest: dict, repository_root: Path) -> list[CuratedSkill]:
    if not isinstance(manifest, dict):
        raise SystemExit("curation manifest must be a JSON object")
    if manifest.get("schemaVersion") != 2:
        raise SystemExit("curation manifest schemaVersion must be 2")
    allowed_statuses = manifest.get("statuses")
    if (
        not isinstance(allowed_statuses, list)
        or len(allowed_statuses) != len(CURATION_STATUSES)
        or not all(isinstance(value, str) for value in allowed_statuses)
        or set(allowed_statuses) != CURATION_STATUSES
    ):
        raise SystemExit("curation manifest statuses must be keep, hold, alias, and omit")
    entries = manifest.get("skills")
    if not isinstance(entries, list):
        raise SystemExit("curation manifest skills must be an array")

    skills_root = repository_root / "skills"
    discovered_paths = {
        path.parent.relative_to(repository_root).as_posix()
        for path in skills_root.rglob("SKILL.md")
    } if skills_root.is_dir() else set()
    seen_paths: set[str] = set()
    by_path: dict[str, dict] = {}
    by_public_name: dict[str, dict] = {}
    for entry in entries:
        if not isinstance(entry, dict):
            raise SystemExit("curation manifest entries must be objects")
        path = entry.get("path")
        if not isinstance(path, str) or not path or Path(path).is_absolute() or ".." in Path(path).parts:
            raise SystemExit(f"curation manifest has invalid skill path: {path!r}")
        if path in seen_paths:
            raise SystemExit(f"curation manifest has duplicate skill path: {path}")
        seen_paths.add(path)
        by_path[path] = entry
        status = entry.get("status")
        if status not in allowed_statuses or status not in CURATION_STATUSES:
            raise SystemExit(f"curation manifest has invalid status {status!r} for {path}")
        public_name = entry.get("publicName")
        if status in PACKAGED_STATUSES:
            if not isinstance(public_name, str) or not PUBLIC_NAME_PATTERN.fullmatch(public_name):
                raise SystemExit(f"packaged curation entry needs a valid publicName: {path}")
            if public_name in by_public_name:
                raise SystemExit(f"curation manifest has duplicate public name: {public_name}")
            by_public_name[public_name] = entry
            if status == "keep" and entry.get("aliasOf") is not None:
                raise SystemExit(f"canonical curation entry cannot define aliasOf: {path}")
        elif public_name is not None or entry.get("aliasOf") is not None:
            raise SystemExit(f"non-packaged curation entry cannot define publicName or aliasOf: {path}")

        dependencies = entry.get("dependencies", [])
        if not isinstance(dependencies, list) or not all(isinstance(value, str) for value in dependencies):
            raise SystemExit(f"curation manifest dependencies must be strings: {path}")

    missing_entries = sorted(discovered_paths - seen_paths)
    missing_source_paths = sorted(seen_paths - discovered_paths)
    if missing_entries:
        raise SystemExit("curation manifest is missing source paths: " + ", ".join(missing_entries))
    if missing_source_paths:
        raise SystemExit("curation manifest references missing source paths: " + ", ".join(missing_source_paths))
    if len(seen_paths) != len(entries):
        raise SystemExit("curation manifest contains duplicate skill paths")

    for path, entry in by_path.items():
        for dependency in entry.get("dependencies", []):
            if dependency not in by_path:
                raise SystemExit(f"curation manifest dependency does not name a source path: {path} -> {dependency}")

    def resolve_target(target: str) -> dict | None:
        if target in by_path:
            return by_path[target]
        return by_public_name.get(target)

    visiting: set[str] = set()
    resolved: set[str] = set()

    def visit(path: str) -> None:
        if path in resolved:
            return
        if path in visiting:
            raise SystemExit(f"curation manifest alias cycle includes: {path}")
        visiting.add(path)
        entry = by_path[path]
        if entry["status"] == "alias":
            target = entry.get("aliasOf")
            if not isinstance(target, str) or not target:
                raise SystemExit(f"alias curation entry needs aliasOf: {path}")
            target_entry = resolve_target(target)
            if target_entry is None:
                raise SystemExit(f"alias targets an unknown skill: {path} -> {target}")
            target_path = next(
                candidate_path for candidate_path, candidate in by_path.items() if candidate is target_entry
            )
            if target_entry["status"] not in PACKAGED_STATUSES:
                raise SystemExit(f"alias targets a non-packaged skill: {path} -> {target}")
            visit(target_path)
        visiting.remove(path)
        resolved.add(path)

    for path in by_path:
        visit(path)

    return [
        CuratedSkill(
            source_path=entry["path"],
            public_name=entry["publicName"],
            alias_of=entry.get("aliasOf"),
        )
        for entry in entries
        if entry["status"] in PACKAGED_STATUSES
    ]


def write_selection_manifest(output: Path, source_spec: SkillSource) -> None:
    records = []
    for item in source_spec.selection:
        records.append(
            {
                "sourcePath": item.source_path,
                "publicName": item.public_name,
                "aliasOf": item.alias_of,
                "repository": source_spec.repository,
                "commit": source_spec.commit,
            }
        )
    manifest = {
        "$schema": "https://github.com/averyfreeman/git-bbq/blob/main/schemas/gitbbq/matt-skills-manifest.schema.json",
        "repository": source_spec.repository,
        "commit": source_spec.commit,
        "skills": records,
    }
    path = output / SELECTION_MANIFEST_FILENAME
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")


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


def rewrite_skill_references(content: str, name_map: dict[str, str]) -> str:
    for source_name in sorted(name_map, key=len, reverse=True):
        public_name = name_map[source_name]
        invocation = re.compile(
            rf"(?P<prefix>[$/]){re.escape(source_name)}(?![A-Za-z0-9_-])",
            re.IGNORECASE,
        )
        content = invocation.sub(
            lambda match: match.group("prefix") + public_name,
            content,
        )
        quoted = re.compile(
            rf"(?P<quote>[`\"']){re.escape(source_name)}(?P=quote)",
            re.IGNORECASE,
        )
        content = quoted.sub(lambda match: match.group("quote") + public_name + match.group("quote"), content)
    return content


def sanitize_skill_directory(skill_directory: Path, name_map: dict[str, str]) -> None:
    for path in skill_directory.rglob("*"):
        if not path.is_file() or path.suffix.lower() not in TEXT_SUFFIXES:
            continue
        try:
            content = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        sanitized = rewrite_skill_references(sanitize_text(content), name_map)
        if sanitized != content:
            path.write_text(sanitized, encoding="utf-8")


def rewrite_text_file(path: Path, replacements: tuple[tuple[str, str], ...]) -> None:
    content = path.read_text(encoding="utf-8")
    for source, replacement in replacements:
        content = content.replace(source, replacement)
    path.write_text(content, encoding="utf-8")


def set_skill_frontmatter_name(path: Path, public_name: str) -> None:
    content = path.read_text(encoding="utf-8")
    updated, count = re.subn(
        r"(?m)^name:\s*[^\n]+$",
        f"name: {public_name}",
        content,
        count=1,
    )
    if count != 1:
        raise SystemExit(f"skill frontmatter has no name field: {path}")
    path.write_text(updated, encoding="utf-8")


def set_skill_public_metadata(skill_directory: Path, public_name: str) -> None:
    path = skill_directory / "agents/openai.yaml"
    if not path.is_file():
        return
    content = path.read_text(encoding="utf-8")
    updated, count = re.subn(
        r"(?m)^(\s*display_name:\s*).+$",
        rf'\1"{public_name}"',
        content,
        count=1,
    )
    if count == 1:
        path.write_text(updated, encoding="utf-8")


def rewrite_curated_skill(relative_path: str, destination: Path, public_name: str) -> None:
    if relative_path.startswith("skills/"):
        relative_path = relative_path.removeprefix("skills/")
    if relative_path == "engineering/ask-matt":
        template_root = ROOT / "adapters/codex/templates/matt/ask-matt"
        copy_file(template_root / "SKILL.md", destination / "SKILL.md")
        copy_file(template_root / "PHASE-BOUNDARIES.md", destination / "PHASE-BOUNDARIES.md")

    if relative_path == "engineering/code-review":
        rewrite_text_file(
            destination / "SKILL.md",
            (
                (
                    "Runs both reviews in parallel sub-agents and reports them side by side.",
                    "Runs both reviews in parallel when available, with a sequential or inline fallback, and reports them side by side.",
                ),
                (
                    "Both axes run as **parallel sub-agents** so they don't pollute each other's context, then this skill aggregates their findings.",
                    "Run both axes in parallel sub-agents when available. If parallel orchestration is unavailable, run them sequentially or inline. In every mode, aggregate the same side-by-side report so the Standards and Spec contract is unchanged.",
                ),
                (
                    "The issue tracker should have been provided to you.",
                    "Use the spec source available in the repository or supplied by the user. If no spec is available, report that limitation and keep the Spec axis separate from Standards.",
                ),
                (
                    "If `docs/agents/issue-tracker.md` is missing, tell the user to run `/setup-matt-pocock-skills`.",
                    "Use a path the user supplied as the spec source, or an ADR/spec under `docs/`, `specs/`, or `.scratch/`. If nothing is available, report `no spec available` and keep the Spec axis separate from Standards.",
                ),
                (
                    "1. Issue references in the commit messages (`#123`, `Closes #45`, GitLab `!67`, etc.), fetched via the workflow in `docs/agents/issue-tracker.md`.\n2. A path the user passed as an argument.\n3. A spec file under `docs/`, `specs/`, or `.scratch/` matching the branch name or feature.\n4. If nothing is found, ask the user where the spec is. If they say there isn't one, the **Spec** sub-agent will skip and report \"no spec available\".",
                    "1. A path the user supplied as the spec source.\n2. An ADR or spec under `docs/`, `specs/`, or `.scratch/` matching the branch or feature.\n3. A relevant locally available commit-message reference.\n4. If nothing is found, report \"no spec available\" and keep the Spec axis separate from Standards.",
                ),
                (
                    "If `docs/agents/issue-tracker.md` is missing, tell the user to run `/setup-matt-pocock-skills`.",
                    "If no spec is available, report that limitation and keep the Spec axis separate from Standards.",
                ),
                (
                    "Use the spec source available in the repository or supplied by the user. If no spec is available, report that limitation and keep the Spec axis separate from Standards. Use a path the user supplied as the spec source, or an ADR/spec under `docs/`, `specs/`, or `.scratch/`. If nothing is available, report `no spec available` and keep the Spec axis separate from Standards.",
                    "Use the spec source available in the repository or supplied by the user. If no spec is available, report that limitation and keep the Spec axis separate from Standards.",
                ),
            ),
        )

    if relative_path == "engineering/codebase-design":
        rewrite_text_file(
            destination / "SKILL.md",
            (
                (
                    "Exploring alternative interfaces**, see [DESIGN-IT-TWICE.md](DESIGN-IT-TWICE.md): spin up parallel sub-agents to design the interface several radically different ways, then compare on depth, locality, and seam placement.",
                    "Exploring alternative interfaces**, see [DESIGN-IT-TWICE.md](DESIGN-IT-TWICE.md): use parallel sub-agents when available, or run the independent designs sequentially or inline, then compare on depth, locality, and seam placement.",
                ),
            ),
        )
        rewrite_text_file(
            destination / "DESIGN-IT-TWICE.md",
            (
                (
                    "When the user wants to explore alternative interfaces for a chosen deepening candidate, use this parallel sub-agent pattern.",
                    "When the user wants to explore alternative interfaces for a chosen deepening candidate, use parallel sub-agents when available; otherwise run the same independent design briefs sequentially or inline. Preserve the same five-part output for every design.",
                ),
                (
                    "Before spawning sub-agents, write a user-facing explanation of the problem space for the chosen candidate:",
                    "Before running the alternative designs, write a user-facing explanation of the problem space for the chosen candidate:",
                ),
                (
                    "Show this to the user, then immediately proceed to Step 2. The user reads and thinks while the sub-agents work in parallel.",
                    "Show this to the user, then immediately proceed to Step 2. Parallel workers may run while the user reads; if the host cannot orchestrate them, run the same briefs sequentially or inline.",
                ),
                ("### 2. Spawn sub-agents", "### 2. Run the design alternatives"),
                (
                    "Spawn 3+ sub-agents in parallel. Each must produce a **radically different** interface for the deepened module.",
                    "When parallel orchestration is available, run 3+ sub-agents in parallel. Otherwise run 3+ independent briefs sequentially or inline. Each must produce a **radically different** interface for the deepened module.",
                ),
                ("Prompt each sub-agent with a separate technical brief", "Give each design a separate technical brief"),
                ("Each sub-agent outputs:", "Each design outputs:"),
            ),
        )

    if relative_path == "productivity/grilling":
        rewrite_text_file(
            destination / "SKILL.md",
            (
                (
                    "Finding _facts_ is your job, never the user's. When a frontier question needs a fact from the environment (filesystem, tools, etc.), dispatch a sub-agent to find it; don't ask the user for anything you could look up yourself. Don't block on it: a running exploration is an unsettled prerequisite, so only the questions downstream of it wait for the sub-agent to report; ask the rest of the frontier now. The _decisions_ are the user's: put each to them and wait.",
                    "Finding _facts_ is your job, never the user's. When a frontier question needs a fact from the environment (filesystem, tools, etc.), dispatch a sub-agent when the host provides one. If sub-agent orchestration is unavailable, perform the same bounded fact-finding inline or sequentially; never ask the user for anything you could look up yourself. Don't block on it: a running exploration is an unsettled prerequisite, so only the questions downstream of it wait for the result; ask the rest of the frontier now. The _decisions_ are the user's: put each to them and wait.",
                ),
            ),
        )

    if relative_path == "engineering/diagnosing-bugs":
        rewrite_text_file(
            destination / "SKILL.md",
            (("`/improve-codebase-architecture`", "a later architecture review"),),
        )

    if relative_path == "engineering/research":
        rewrite_text_file(
            destination / "SKILL.md",
            (
                (
                    "Spin up a **background agent** to do the research, so you keep working while it reads.",
                    "Run the research in a background agent when the host provides one. If background orchestration is unavailable, run the same work inline or sequentially. In every mode, preserve the same single cited Markdown artifact contract.",
                ),
                ("Its job:", "The research worker's job:"),
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

    set_skill_frontmatter_name(destination / "SKILL.md", public_name)
    set_skill_public_metadata(destination, public_name)


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
