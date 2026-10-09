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
ONBOARDING_SKILL = "./skills/setup-matt-pocock-skills/SKILL.md"
SKILLS_REPOSITORY = "https://github.com/mattpocock/skills.git"
SKILL_NAME_PATTERN = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
ALLOWED_FRONTMATTER = {"name", "description", "license", "compatibility", "metadata", "allowed-tools"}


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
    if repository != SKILLS_REPOSITORY:
        raise ValueError("packaged skills must come directly from mattpocock/skills")
    if not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("packaged Matt skills manifest has an invalid commit")
    if not isinstance(records, list) or not records:
        raise ValueError("packaged Matt skills manifest must contain skills")

    names: set[str] = set()
    source_paths: set[str] = set()
    for record in records:
        if not isinstance(record, dict):
            raise ValueError("packaged Matt skills manifest entries must be objects")
        if not {"sourcePath", "name", "repository", "commit"}.issubset(record):
            raise ValueError(f"packaged Matt skills manifest entry is incomplete: {record!r}")
        if record["repository"] != repository or record["commit"] != commit:
            raise ValueError("packaged Matt skills entries disagree with the source pin")
        source_path = record["sourcePath"]
        if (
            not isinstance(source_path, str)
            or not source_path.startswith("skills/")
            or Path(source_path).is_absolute()
            or ".." in Path(source_path).parts
            or Path(source_path).as_posix() != source_path
            or len(Path(source_path).parts) < 3
        ):
            raise ValueError(f"packaged Matt skills manifest has an invalid source path: {source_path!r}")
        if source_path in source_paths:
            raise ValueError(f"packaged Matt skills manifest has a duplicate source path: {source_path}")
        source_paths.add(source_path)
        name = record["name"]
        if not isinstance(name, str) or not SKILL_NAME_PATTERN.fullmatch(name):
            raise ValueError(f"packaged Matt skills manifest has an invalid name: {name!r}")
        if name != Path(source_path).name:
            raise ValueError(f"packaged skill name does not preserve its upstream path: {source_path}")
        if name in names:
            raise ValueError(f"packaged Matt skills manifest has a duplicate name: {name}")
        names.add(name)
    return manifest, records


def parse_skill_frontmatter(path: Path) -> tuple[dict[str, str], list[str]]:
    errors: list[str] = []
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as exc:
        return {}, [f"could not read skill: {path}: {exc}"]
    match = re.match(r"\A---\r?\n(?P<frontmatter>.*?)\r?\n---(?:\r?\n|\Z)", text, re.DOTALL)
    if match is None:
        return {}, [f"skill has invalid YAML frontmatter: {path}"]

    fields: dict[str, str] = {}
    lines = match.group("frontmatter").splitlines()
    for line in lines:
        if not line or line[0].isspace() or line.lstrip().startswith("#"):
            continue
        field = re.match(r"^([A-Za-z0-9_-]+):(?:\s*(.*))?$", line)
        if field is None:
            errors.append(f"skill has an invalid top-level frontmatter line: {path}: {line}")
            continue
        key = field.group(1)
        value = (field.group(2) or "").strip()
        if key not in ALLOWED_FRONTMATTER:
            errors.append(f"skill uses a nonstandard frontmatter field {key!r}: {path}")
        if key in fields:
            errors.append(f"skill repeats frontmatter field {key!r}: {path}")
        fields[key] = value

    description = fields.get("description", "")
    if description in {">", "|", ">-", "|-", ">+", "|+"}:
        description = " ".join(line.strip() for line in lines[lines.index(f"description: {description}") + 1:] if line.strip())
    description = description.strip("\"'")
    fields["description"] = description
    if not fields.get("name"):
        errors.append(f"skill is missing required name frontmatter: {path}")
    if not fields.get("description"):
        errors.append(f"skill is missing required description frontmatter: {path}")
    if len(description) > 1024:
        errors.append(f"skill description exceeds 1024 characters: {path}")
    return fields, errors


def require(condition: bool, message: str, errors: list[str]) -> None:
    if not condition:
        errors.append(message)


def object_field(parent: dict, key: str, context: str, errors: list[str]) -> dict:
    value = parent.get(key, {})
    if not isinstance(value, dict):
        errors.append(f"{context}.{key} must be an object")
        return {}
    return value


def validate(
    package: Path,
    target: str | None,
    variant: str = "local",
    warnings: list[str] | None = None,
) -> list[str]:
    del warnings  # The validator reports conformance errors; upstream wording remains untouched.
    errors: list[str] = []
    if variant not in {"local", "public"}:
        return [f"unsupported plugin variant {variant!r}; choose local or public"]
    portable = load_json(package / "plugin.json")
    compatibility = load_json(package / ".codex-plugin/plugin.json")
    _selection_manifest, selection = load_selection_manifest(package)

    require(portable.get("name") == "git-bbq", "portable name must be git-bbq", errors)
    require(
        portable.get("$schema") == "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
        "portable manifest must declare the Agent Plugins schema",
        errors,
    )
    extensions = object_field(portable, "extensions", "plugin.json", errors)
    openai = object_field(extensions, "com.openai", "plugin.json.extensions", errors)
    if variant == "local":
        require(openai.get("hooks") == "./hooks/hooks.json", "OpenAI hooks path is incorrect", errors)
        try:
            hooks = load_json(package / "hooks/hooks.json")
        except ValueError as exc:
            hooks = {}
            errors.append(str(exc))
        hook_config = hooks.get("hooks", {})
        if not isinstance(hook_config, dict):
            errors.append("hooks/hooks.json: hooks must be an object")
            hook_config = {}
        hook_events = set(hook_config)
        require(hook_events == REQUIRED_EVENTS, "hook events must be exactly the five Git BBQ events", errors)
    else:
        require("hooks" not in openai, "public manifest must not reference lifecycle hooks", errors)
        require(not (package / "hooks").exists(), "public package must not contain hook files", errors)
    onboarding_skill = openai.get("onboardingSkill")
    require(
        onboarding_skill == ONBOARDING_SKILL,
        "OpenAI onboardingSkill must point to the bundled setup-matt-pocock-skills skill",
        errors,
    )
    require(
        (package / ONBOARDING_SKILL).is_file(),
        "OpenAI onboardingSkill does not resolve to a packaged skill",
        errors,
    )
    interface = object_field(openai, "interface", "plugin.json.extensions.com.openai", errors)
    for field in ("displayName", "shortDescription", "longDescription", "developerName", "category"):
        require(bool(interface.get(field)), f"OpenAI interface field is missing: {field}", errors)
    short_description = interface.get("shortDescription", "")
    require(len(short_description) <= 30, "OpenAI shortDescription exceeds 30 characters", errors)
    prompts = interface.get("defaultPrompt", [])
    require(isinstance(prompts, list) and len(prompts) <= 3, "OpenAI defaultPrompt must have at most three entries", errors)
    require(compatibility.get("name") == "git-bbq", "compatibility name must be git-bbq", errors)
    require("hooks" not in compatibility, "compatibility manifest must use hook autodiscovery", errors)
    compatibility_extensions = object_field(compatibility, "extensions", ".codex-plugin/plugin.json", errors)
    compatibility_openai = object_field(
        compatibility_extensions,
        "com.openai",
        ".codex-plugin/plugin.json.extensions",
        errors,
    )
    require(
        compatibility_openai.get("onboardingSkill") == onboarding_skill,
        "Codex compatibility manifest must declare the same onboardingSkill",
        errors,
    )
    require(
        "hooks" not in compatibility_openai,
        "compatibility manifest must use hook autodiscovery",
        errors,
    )
    compatibility_author = object_field(compatibility, "author", ".codex-plugin/plugin.json", errors)
    compatibility_interface = object_field(compatibility, "interface", ".codex-plugin/plugin.json", errors)
    require(bool(compatibility_author.get("name")), "compatibility author is missing", errors)
    require(bool(compatibility_interface.get("developerName")), "compatibility developerName is missing", errors)

    if variant == "public":
        for manifest_path in (package / "plugin.json", package / ".codex-plugin/plugin.json"):
            try:
                manifest_text = manifest_path.read_text(encoding="utf-8")
            except OSError:
                continue
            require('"hooks"' not in manifest_text, f"public manifest contains a hook reference: {manifest_path.name}", errors)
    require((package / "assets/logo.svg").is_file(), "logo asset is missing", errors)
    require((package / "licenses/mattpocock-skills/LICENSE").is_file(), "upstream skill license is missing", errors)
    require((package / "runtime/git-bbq/git-bbq").is_file(), "POSIX launcher is missing", errors)
    require((package / "runtime/git-bbq/git-bbq.cmd").is_file(), "Windows launcher is missing", errors)

    skills_root = package / "skills"
    skill_dirs = [path for path in skills_root.iterdir() if path.is_dir()] if skills_root.is_dir() else []
    expected_skills = {"git-bbq", *(record["name"] for record in selection)}
    actual_skills = {path.name for path in skill_dirs}
    require(actual_skills == expected_skills, "packaged skills do not match the root curation selection", errors)
    for skill_dir in skill_dirs:
        skill_path = skill_dir / "SKILL.md"
        require(skill_path.is_file(), f"skill is missing SKILL.md: {skill_dir.name}", errors)
        if not skill_path.is_file():
            continue
        fields, frontmatter_errors = parse_skill_frontmatter(skill_path)
        errors.extend(frontmatter_errors)
        name = fields.get("name", "").strip("\"'")
        require(name == skill_dir.name, f"skill name must match its parent directory: {skill_dir.name}", errors)
        require(len(name) <= 64, f"skill name exceeds 64 characters: {skill_dir.name}", errors)
        require("--" not in name, f"skill name contains consecutive hyphens: {skill_dir.name}", errors)
    if variant == "public":
        try:
            builtin_skill = (skills_root / "git-bbq/SKILL.md").read_text(encoding="utf-8")
        except OSError:
            builtin_skill = ""
        require(
            "git-bbq help hooks" not in builtin_skill and "local-hooks:" not in builtin_skill,
            "public Git BBQ skill must omit local hook instructions",
            errors,
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
    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("package", type=Path)
    parser.add_argument(
        "--target",
        choices=["all", "aarch64-darwin", "x86_64-darwin", "x86_64-linux", "windows-x86_64"],
    )
    parser.add_argument("--variant", choices=["local", "public"], default="local")
    args = parser.parse_args()
    try:
        errors = validate(args.package.resolve(), args.target, args.variant)
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
