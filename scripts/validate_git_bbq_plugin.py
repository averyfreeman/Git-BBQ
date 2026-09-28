#!/usr/bin/env python3
"""Validate the generated Git BBQ portable and Codex plugin package."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

REQUIRED_EVENTS = {"UserPromptSubmit", "PreToolUse", "PostToolUse", "PostCompact", "Stop"}
SELECTION_MANIFEST = Path("skills/matt-skills-manifest.json")
PUBLIC_NAME_PATTERN = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
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


def load_selection_manifest(package: Path) -> tuple[dict, list[dict]]:
    manifest = load_json(package / SELECTION_MANIFEST)
    repository = manifest.get("repository")
    commit = manifest.get("commit")
    records = manifest.get("skills")
    if not isinstance(repository, str) or not repository:
        raise ValueError("packaged Matt skills manifest is missing repository")
    if not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("packaged Matt skills manifest has an invalid commit")
    if not isinstance(records, list) or not records:
        raise ValueError("packaged Matt skills manifest must contain skills")
    public_names: set[str] = set()
    source_paths: set[str] = set()
    for record in records:
        if not isinstance(record, dict):
            raise ValueError("packaged Matt skills manifest entries must be objects")
        required = {"sourcePath", "publicName", "aliasOf", "repository", "commit"}
        if not required.issubset(record):
            raise ValueError(f"packaged Matt skills manifest entry is incomplete: {record!r}")
        if record["repository"] != repository or record["commit"] != commit:
            raise ValueError("packaged Matt skills manifest entries disagree with its source pin")
        source_path = record["sourcePath"]
        if (
            not isinstance(source_path, str)
            or not source_path.startswith("skills/")
            or Path(source_path).is_absolute()
            or ".." in Path(source_path).parts
        ):
            raise ValueError(f"packaged Matt skills manifest has an invalid source path: {source_path!r}")
        if source_path in source_paths:
            raise ValueError(f"packaged Matt skills manifest has a duplicate source path: {source_path}")
        source_paths.add(source_path)
        public_name = record["publicName"]
        if not isinstance(public_name, str) or not PUBLIC_NAME_PATTERN.fullmatch(public_name):
            raise ValueError(f"packaged Matt skills manifest has an invalid public name: {public_name!r}")
        if public_name in public_names:
            raise ValueError(f"packaged Matt skills manifest has duplicate public name: {public_name}")
        public_names.add(public_name)
    for record in records:
        alias_of = record["aliasOf"]
        if alias_of is not None and (not isinstance(alias_of, str) or alias_of not in public_names):
            raise ValueError(
                f"packaged Matt skills manifest alias targets a non-packaged skill: {alias_of}"
            )
    return manifest, records


def require(condition: bool, message: str, errors: list[str]) -> None:
    if not condition:
        errors.append(message)


def warn(condition: bool, message: str, warnings: list[str]) -> None:
    if condition:
        warnings.append(message)


def validate(
    package: Path,
    target: str | None,
    warnings: list[str] | None = None,
) -> list[str]:
    errors: list[str] = []
    if warnings is None:
        warnings = []
    portable_path = package / "plugin.json"
    compatibility_path = package / ".codex-plugin/plugin.json"
    hooks_path = package / "hooks/hooks.json"
    portable = load_json(portable_path)
    compatibility = load_json(compatibility_path)
    hooks = load_json(hooks_path)
    _selection_manifest, selection = load_selection_manifest(package)

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
    expected_skills = {"git-bbq"} | {
        f"mattpocock-{record['publicName']}" for record in selection
    }
    require(actual_skills == expected_skills, "packaged skills do not match the curated selection manifest", errors)
    require(len(selection) == 16, "packaged Matt skills must contain exactly 16 selected entries", errors)
    for skill_dir in skill_dirs:
        require((skill_dir / "SKILL.md").is_file(), f"skill is missing SKILL.md: {skill_dir.name}", errors)

    source_names = {
        Path(record["sourcePath"]).name: record["publicName"]
        for record in selection
    }
    for record in selection:
        directory = skills_root / f"mattpocock-{record['publicName']}"
        skill_path = directory / "SKILL.md"
        try:
            skill_text = skill_path.read_text(encoding="utf-8")
        except OSError as exc:
            errors.append(f"could not read packaged skill frontmatter {skill_path}: {exc}")
            continue
        frontmatter = re.search(r"(?m)^name:\s*([^\n]+)$", skill_text)
        require(frontmatter is not None, f"skill frontmatter has no name: {directory.name}", errors)
        if frontmatter:
            require(
                frontmatter.group(1).strip() == record["publicName"],
                f"skill frontmatter name is not canonical: {directory.name}",
                errors,
            )
        for source_name, public_name in source_names.items():
            if source_name == public_name:
                continue
            old_reference = re.compile(
                rf"(?:\$|/){re.escape(source_name)}(?:\b|`|\")",
                re.IGNORECASE,
            )
            if old_reference.search(skill_text):
                errors.append(
                    f"obsolete cross-skill reference {source_name!r} found in {directory.name}"
                )

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
                warn(
                    bool(OLD_BRANDING.search(content)),
                    f"obsolete branding found in package file: {relative}",
                    warnings,
                )
                warn(
                    bool(FORBIDDEN_PROVIDER_BRANDING.search(content)),
                    f"provider-specific branding found in package file: {relative}",
                    warnings,
                )
                if FORBIDDEN_CLAUDE_METADATA.search(content):
                    errors.append(f"legacy invocation metadata found in package file: {relative}")
                if path.suffix.lower() in TEXT_SUFFIXES:
                    warn(
                        bool(BRITISH_SPELLINGS.search(content)),
                        f"non-US spelling found in package file: {relative}",
                        warnings,
                    )
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
        warnings: list[str] = []
        errors = validate(args.package.resolve(), args.target, warnings)
    except ValueError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1
    for warning in warnings:
        print(f"warning: {warning}", file=sys.stderr)
    if errors:
        for error in errors:
            print(f"error: {error}", file=sys.stderr)
        return 1
    print(f"Git BBQ plugin package is valid: {args.package}")
    if warnings:
        print(f"Git BBQ plugin package has {len(warnings)} warning(s)", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
