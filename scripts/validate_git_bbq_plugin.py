#!/usr/bin/env python3
"""Validate the generated Git BBQ portable and Codex plugin package."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

REQUIRED_EVENTS = {"UserPromptSubmit", "PreToolUse", "PostToolUse", "PostCompact", "Stop"}
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
EXPECTED_SKILLS = {"git-bbq"} | {
    f"mattpocock-{'-'.join(path.split('/'))}" for path in MATT_SKILL_PATHS
}
_AI = bytes((97, 105))
_OLD_BRANDING_PATTERN = b"|".join(
    (
        _AI + rb"[-_ ]" + b"software" + rb"[-_ ]" + b"architect",
        _AI + rb"[-_ ]" + b"architect",
        _AI + b"architect",
        _AI + b"-software-architect",
    )
)
OLD_BRANDING = re.compile(_OLD_BRANDING_PATTERN, re.IGNORECASE)
FORBIDDEN_PROVIDER_BRANDING = re.compile(rb"\b(?:claude|anthropic)\b", re.IGNORECASE)
FORBIDDEN_CLAUDE_METADATA = re.compile(rb"disable-model-invocation", re.IGNORECASE)
BRITISH_SPELLINGS = re.compile(
    rb"\b(?:behaviour|behaviours|colour|colours|centre|centres|labelled|labelling|modelling|"
    rb"optimise|optimised|optimising|prioritise|prioritised|prioritising|recognised|"
    rb"summarise|summarised|summarising|travelling|artefact|artefacts|authorise|"
    rb"authorised|authorising|minimise|minimised|minimising|organisation|organisations|"
    rb"programme|programmes|favour|favours|licence)\b",
    re.IGNORECASE,
)
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


def load_json(path: Path) -> dict:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise ValueError(f"{path}: invalid JSON: {exc}") from exc
    if not isinstance(value, dict):
        raise ValueError(f"{path}: expected a JSON object")
    return value


def require(condition: bool, message: str, errors: list[str]) -> None:
    if not condition:
        errors.append(message)


def validate(package: Path, target: str | None) -> list[str]:
    errors: list[str] = []
    portable_path = package / "plugin.json"
    compatibility_path = package / ".codex-plugin/plugin.json"
    hooks_path = package / "hooks/hooks.json"
    portable = load_json(portable_path)
    compatibility = load_json(compatibility_path)
    hooks = load_json(hooks_path)

    require(portable.get("name") == "git-bbq", "portable name must be git-bbq", errors)
    require(
        portable.get("$schema") == "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
        "portable manifest must declare the Agent Plugins schema",
        errors,
    )
    openai = portable.get("extensions", {}).get("com.openai", {})
    require(openai.get("hooks") == "./hooks/hooks.json", "OpenAI hooks path is incorrect", errors)
    interface = openai.get("interface", {})
    for field in ("displayName", "shortDescription", "longDescription", "developerName", "category"):
        require(bool(interface.get(field)), f"OpenAI interface field is missing: {field}", errors)
    require(compatibility.get("name") == "git-bbq", "compatibility name must be git-bbq", errors)
    require("hooks" not in compatibility, "compatibility manifest must use hook autodiscovery", errors)
    require("extensions" not in compatibility, "compatibility manifest must not define extensions", errors)
    require(bool(compatibility.get("author", {}).get("name")), "compatibility author is missing", errors)
    require(bool(compatibility.get("interface", {}).get("developerName")), "compatibility developerName is missing", errors)

    hook_events = set(hooks.get("hooks", {}))
    require(hook_events == REQUIRED_EVENTS, "hook events must be exactly the five Git BBQ events", errors)
    require((package / "assets/logo.svg").is_file(), "logo asset is missing", errors)
    require((package / "runtime/git-bbq/git-bbq").is_file(), "POSIX launcher is missing", errors)
    require((package / "runtime/git-bbq/git-bbq.cmd").is_file(), "Windows launcher is missing", errors)

    skills_root = package / "skills"
    skill_dirs = [path for path in skills_root.iterdir() if path.is_dir()] if skills_root.is_dir() else []
    require(skill_dirs, "no direct skill directories were packaged", errors)
    actual_skills = {path.name for path in skill_dirs}
    require(actual_skills == EXPECTED_SKILLS, "packaged skills do not match the curated allowlist", errors)
    for skill_dir in skill_dirs:
        require((skill_dir / "SKILL.md").is_file(), f"skill is missing SKILL.md: {skill_dir.name}", errors)

    runtime_root = package / "runtime/bin"
    runtime_targets = [path for path in runtime_root.iterdir() if path.is_dir()] if runtime_root.is_dir() else []
    require(runtime_targets, "no runtime target was packaged", errors)
    if target and target != "all":
        executable = "git-bbq.exe" if target == "windows-x86_64" else "git-bbq"
        require((runtime_root / target / executable).is_file(), f"runtime target is missing: {target}", errors)
    if target == "all":
        for expected in ("aarch64-darwin", "x86_64-darwin", "x86_64-linux", "windows-x86_64"):
            executable = "git-bbq.exe" if expected == "windows-x86_64" else "git-bbq"
            require((runtime_root / expected / executable).is_file(), f"runtime target is missing: {expected}", errors)

    for path in package.rglob("*"):
        if path.is_file():
            try:
                relative = path.relative_to(package)
                if any(term in str(relative).lower() for term in ("claude", "anthropic")):
                    errors.append(f"obsolete provider path found in package: {relative}")
                content = path.read_bytes()
                if OLD_BRANDING.search(content):
                    errors.append(f"obsolete branding found in package file: {path.relative_to(package)}")
                if FORBIDDEN_PROVIDER_BRANDING.search(content):
                    errors.append(f"provider-specific branding found in package file: {relative}")
                if FORBIDDEN_CLAUDE_METADATA.search(content):
                    errors.append(f"legacy invocation metadata found in package file: {relative}")
                if path.suffix.lower() in TEXT_SUFFIXES and BRITISH_SPELLINGS.search(content):
                    errors.append(f"non-US spelling found in package file: {relative}")
            except OSError as exc:
                errors.append(f"could not read package file {path}: {exc}")
    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("package", type=Path)
    parser.add_argument(
        "--target",
        choices=["all", "aarch64-darwin", "x86_64-darwin", "x86_64-linux", "windows-x86_64"],
    )
    args = parser.parse_args()
    try:
        errors = validate(args.package.resolve(), args.target)
    except ValueError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1
    if errors:
        for error in errors:
            print(f"error: {error}", file=sys.stderr)
        return 1
    print(f"Git BBQ plugin package is valid: {args.package}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
