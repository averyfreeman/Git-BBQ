#!/usr/bin/env python3
"""Build the standalone Git BBQ Codex plugin from the Go runtime."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
import io
import json
import os
import platform
import re
import shutil
import subprocess
import sys
import tempfile
import tarfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_VERSION = "0.4.0"
SKILLS_REPOSITORY = "https://github.com/mattpocock/skills.git"
CURATION_FILENAME = "git-bbq-curation.json"
SELECTION_MANIFEST_FILENAME = "skills/matt-skills-manifest.json"
SKILL_NAME_PATTERN = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
STABLE_SKILL_CATEGORIES = ("engineering", "productivity")
TARGETS = {
    "windows-x86_64": ("windows", "amd64", "git-bbq.exe"),
    "aarch64-darwin": ("darwin", "arm64", "git-bbq"),
    "x86_64-darwin": ("darwin", "amd64", "git-bbq"),
    "x86_64-linux": ("linux", "amd64", "git-bbq"),
}


@dataclass(frozen=True)
class CuratedSkill:
    """Describe an unchanged upstream skill selected for packaging."""

    source_path: str
    name: str


@dataclass(frozen=True)
class SkillSource:
    """Describe the upstream pin and root-owned package selection."""

    repository: str
    commit: str
    checkout: Path
    selection: tuple[CuratedSkill, ...]


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


def git_output(path: Path, *arguments: str) -> str | None:
    try:
        result = subprocess.run(
            ["git", "-C", str(path), *arguments],  # noqa: S603, S607
            check=True,
            capture_output=True,
            text=True,
        )
    except (OSError, subprocess.CalledProcessError):
        return None
    return result.stdout.strip()


def discover_stable_skill_paths(repository_root: Path) -> set[str]:
    paths: set[str] = set()
    for category in STABLE_SKILL_CATEGORIES:
        category_root = repository_root / "skills" / category
        if not category_root.is_dir():
            continue
        paths.update(
            child.relative_to(repository_root).as_posix()
            for child in category_root.iterdir()
            if child.is_dir() and (child / "SKILL.md").is_file()
        )
    return paths


def validate_curation(manifest: dict, repository_root: Path) -> list[CuratedSkill]:
    if not isinstance(manifest, dict) or manifest.get("schemaVersion") != 1:
        raise SystemExit("root curation manifest schemaVersion must be 1")
    source = manifest.get("source")
    if not isinstance(source, dict):
        raise SystemExit("root curation manifest must define its upstream source")
    if source.get("repository") != SKILLS_REPOSITORY:
        raise SystemExit(f"skill source must be the upstream repository: {SKILLS_REPOSITORY}")
    commit = source.get("commit")
    if not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise SystemExit("skill source must pin a 40-character commit")

    entries = manifest.get("skills")
    if not isinstance(entries, list) or not entries:
        raise SystemExit("root curation manifest must select at least one skill")

    selection: list[CuratedSkill] = []
    seen_paths: set[str] = set()
    seen_names: set[str] = set()
    for entry in entries:
        if not isinstance(entry, dict):
            raise SystemExit("root curation entries must be objects")
        source_path = entry.get("path")
        name = entry.get("name")
        if (
            not isinstance(source_path, str)
            or not source_path.startswith("skills/")
            or Path(source_path).is_absolute()
            or ".." in Path(source_path).parts
            or Path(source_path).as_posix() != source_path
            or len(Path(source_path).parts) != 3
        ):
            raise SystemExit(f"root curation entry has an invalid upstream path: {source_path!r}")
        if source_path in seen_paths:
            raise SystemExit(f"root curation manifest has a duplicate source path: {source_path}")
        if not isinstance(name, str) or not SKILL_NAME_PATTERN.fullmatch(name):
            raise SystemExit(f"root curation entry has an invalid upstream name: {name!r}")
        if name != Path(source_path).name:
            raise SystemExit(f"skill name must preserve the upstream directory name: {source_path}")
        if name in seen_names:
            raise SystemExit(f"root curation manifest has a duplicate skill name: {name}")
        if not (repository_root / source_path / "SKILL.md").is_file():
            raise SystemExit(f"selected upstream skill is missing SKILL.md: {source_path}")
        seen_paths.add(source_path)
        seen_names.add(name)
        selection.append(CuratedSkill(source_path=source_path, name=name))
    discovered = discover_stable_skill_paths(repository_root)
    selected = {item.source_path for item in selection}
    if selected != discovered:
        missing = sorted(discovered - selected)
        stale = sorted(selected - discovered)
        details = []
        if missing:
            details.append(f"unselected stable skills: {', '.join(missing)}")
        if stale:
            details.append(f"paths outside the stable selection: {', '.join(stale)}")
        raise SystemExit("root curation manifest does not match its stable-skill policy (" + "; ".join(details) + ")")
    return selection


def load_pinned_source(checkout: Path) -> SkillSource:
    manifest_path = ROOT / CURATION_FILENAME
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise SystemExit(f"could not read root skill curation manifest {manifest_path}: {exc}") from exc
    selection = validate_curation(manifest, checkout)
    source = manifest["source"]
    return SkillSource(
        repository=source["repository"],
        commit=source["commit"],
        checkout=checkout,
        selection=tuple(selection),
    )


def copy_matt_skills(output: Path) -> None:
    manifest_path = ROOT / CURATION_FILENAME
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise SystemExit(f"could not read root skill curation manifest {manifest_path}: {exc}") from exc
    source = manifest.get("source", {})
    repository = source.get("repository")
    commit = source.get("commit")
    if repository != SKILLS_REPOSITORY or not isinstance(commit, str):
        raise SystemExit("root curation manifest does not pin the upstream Matt Pocock repository")

    checkout = ROOT / ".agents/skills"
    if checkout.is_dir():
        if git_head(checkout) != commit:
            raise SystemExit(
                f"upstream skill submodule is not at pinned commit {commit}; "
                "run git submodule update --init"
            )
        remote = git_output(checkout, "remote", "get-url", "origin")
        if remote != repository:
            raise SystemExit(f"upstream skill submodule origin must be {repository}; found {remote!r}")
        dirty = git_output(checkout, "status", "--porcelain", "--untracked-files=all")
        if dirty is None:
            raise SystemExit(f"could not inspect upstream skill submodule: {checkout}")
        if dirty:
            raise SystemExit("upstream skill submodule must be clean before package build")
        source_spec = load_pinned_source(checkout)
        with tempfile.TemporaryDirectory(prefix="git-bbq-skills-export-") as temporary:
            exported = export_pinned_selection(source_spec, Path(temporary))
            copy_skill_directories(exported, output)
        return

    with tempfile.TemporaryDirectory(prefix="git-bbq-skills-") as temporary:
        checkout = Path(temporary) / "skills"
        subprocess.run(
            ["git", "clone", "--filter=blob:none", "--no-checkout", repository, str(checkout)],
            cwd=ROOT,
            check=True,
        )
        subprocess.run(
            ["git", "-C", str(checkout), "checkout", "--detach", commit],
            cwd=ROOT,
            check=True,
        )
        if git_head(checkout) != commit:
            raise SystemExit(f"upstream skill checkout is not pinned to {commit}")
        source_spec = load_pinned_source(checkout)
        with tempfile.TemporaryDirectory(prefix="git-bbq-skills-export-") as exported_directory:
            exported = export_pinned_selection(source_spec, Path(exported_directory))
            copy_skill_directories(exported, output)


def export_pinned_selection(source_spec: SkillSource, destination: Path) -> SkillSource:
    """Export only files from the pinned Git tree, never from the worktree."""

    destination.mkdir(parents=True, exist_ok=True)
    paths = [item.source_path for item in source_spec.selection]
    license_in_tree = subprocess.run(
        ["git", "-C", str(source_spec.checkout), "cat-file", "-e", f"{source_spec.commit}:LICENSE"],  # noqa: S603, S607
        check=False,
        capture_output=True,
    )
    if license_in_tree.returncode == 0:
        paths.append("LICENSE")
    try:
        archive = subprocess.run(
            ["git", "-C", str(source_spec.checkout), "archive", "--format=tar", source_spec.commit, *paths],  # noqa: S603, S607
            check=True,
            capture_output=True,
        ).stdout
    except (OSError, subprocess.CalledProcessError) as exc:
        raise SystemExit(f"could not export pinned upstream skills: {exc}") from exc
    try:
        with tarfile.open(fileobj=io.BytesIO(archive), mode="r:") as bundle:
            for member in bundle.getmembers():
                member_path = Path(member.name)
                if member_path.is_absolute() or ".." in member_path.parts:
                    raise SystemExit(f"pinned source archive contains an unsafe path: {member.name}")
            bundle.extractall(destination, filter="data")
    except (tarfile.TarError, OSError) as exc:
        raise SystemExit(f"could not unpack pinned upstream skills: {exc}") from exc
    return SkillSource(
        repository=source_spec.repository,
        commit=source_spec.commit,
        checkout=destination,
        selection=source_spec.selection,
    )


def copy_skill_directories(source_spec: SkillSource, output: Path) -> None:
    destination_root = output / "skills"
    destination_root.mkdir(parents=True, exist_ok=True)
    write_selection_manifest(output, source_spec)
    upstream_license = source_spec.checkout / "LICENSE"
    if upstream_license.is_file():
        copy_file(upstream_license, output / "licenses/mattpocock-skills/LICENSE")
    for item in source_spec.selection:
        source_directory = source_spec.checkout / item.source_path
        destination = destination_root / item.name
        shutil.copytree(source_directory, destination)
        adapt_agent_skills_frontmatter(destination / "SKILL.md", destination)


def copy_gitbbq_skill(output: Path, variant: str) -> None:
    source = ROOT / "adapters/codex/templates/git-bbq-skill.md"
    content = source.read_text(encoding="utf-8")
    marker_start = "<!-- local-hooks:start -->\n"
    marker_end = "<!-- local-hooks:end -->\n"
    if variant == "public":
        start = content.index(marker_start)
        end = content.index(marker_end, start) + len(marker_end)
        content = content[:start] + content[end:]
    else:
        content = content.replace(marker_start, "").replace(marker_end, "")
    destination = output / "skills/git-bbq/SKILL.md"
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(content, encoding="utf-8")


def adapt_agent_skills_frontmatter(skill_path: Path, skill_directory: Path) -> None:
    content = skill_path.read_text(encoding="utf-8")
    match = re.match(r"\A---\r?\n(?P<frontmatter>.*?)\r?\n---(?P<rest>\r?\n.*)\Z", content, re.DOTALL)
    if match is None:
        raise SystemExit(f"upstream skill has invalid YAML frontmatter: {skill_path}")
    frontmatter = match.group("frontmatter")
    manual_only = re.search(r"(?m)^disable-model-invocation:\s*(\S+)\s*$", frontmatter)
    argument_hint = re.search(r"(?m)^argument-hint:\s*(.*?)\s*$", frontmatter)
    if manual_only is None and argument_hint is None:
        return
    if manual_only is not None and manual_only.group(1).lower() != "true":
        raise SystemExit(f"unsupported disable-model-invocation value in {skill_path}")
    if manual_only is not None:
        openai_metadata = skill_directory / "agents/openai.yaml"
        try:
            openai_text = openai_metadata.read_text(encoding="utf-8")
        except OSError as exc:
            raise SystemExit(
                f"manual-only upstream skill lacks Codex invocation policy: {openai_metadata}"
            ) from exc
        if not re.search(r"(?m)^\s+allow_implicit_invocation:\s*false\s*$", openai_text):
            raise SystemExit(f"manual-only upstream skill lacks allow_implicit_invocation: false: {openai_metadata}")
        frontmatter = re.sub(r"(?m)^disable-model-invocation:\s*true\s*\r?\n?", "", frontmatter)
    if argument_hint is not None:
        hint_value = argument_hint.group(1).strip()
        if hint_value.startswith('"') and hint_value.endswith('"'):
            try:
                hint_value = json.loads(hint_value)
            except json.JSONDecodeError as exc:
                raise SystemExit(f"unsupported argument-hint value in {skill_path}") from exc
        elif hint_value.startswith("'") and hint_value.endswith("'"):
            hint_value = hint_value[1:-1].replace("''", "'")
        frontmatter = re.sub(r"(?m)^argument-hint:[ \t]*[^\r\n]*(?:\r?\n|$)", "", frontmatter)
        metadata_header = re.search(r"(?m)^metadata:[ \t]*\r?\n", frontmatter)
        hint_line = f"  git-bbq-argument-hint: {json.dumps(hint_value, ensure_ascii=False)}\n"
        if metadata_header is None:
            frontmatter = f"{frontmatter.rstrip()}\nmetadata:\n{hint_line.rstrip()}"
        else:
            insertion = metadata_header.end()
            for line in frontmatter[insertion:].splitlines(keepends=True):
                if line.strip() and not line[0].isspace():
                    break
                insertion += len(line)
            frontmatter = frontmatter[:insertion] + hint_line + frontmatter[insertion:]
    skill_path.write_text(f"---\n{frontmatter}\n---{match.group('rest')}", encoding="utf-8")


def write_selection_manifest(output: Path, source_spec: SkillSource) -> None:
    records = [
        {
            "sourcePath": item.source_path,
            "name": item.name,
            "repository": source_spec.repository,
            "commit": source_spec.commit,
        }
        for item in source_spec.selection
    ]
    manifest = {
        "$schema": "https://github.com/averyfreeman/git-bbq/blob/main/schemas/gitbbq/matt-skills-manifest.schema.json",
        "repository": source_spec.repository,
        "commit": source_spec.commit,
        "skills": records,
    }
    path = output / SELECTION_MANIFEST_FILENAME
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")


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
    onboarding_skill = extension.get("onboardingSkill")
    if onboarding_skill:
        compatibility["extensions"] = {
            "com.openai": {"onboardingSkill": onboarding_skill}
        }
    return compatibility


def manifest_for_variant(portable: dict, variant: str) -> dict:
    if variant not in {"local", "public"}:
        raise SystemExit(f"unsupported plugin variant {variant!r}; choose local or public")
    manifest = json.loads(json.dumps(portable))
    openai = manifest.get("extensions", {}).get("com.openai", {})
    if variant == "public":
        openai.pop("hooks", None)
        interface = openai.get("interface", {})
        interface["longDescription"] = (
            "Git BBQ provides durable project context, architecture records, "
            "language guidance, and guarded Git workflows through a local Codex CLI."
        )
    return manifest


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
    manifest = manifest_for_variant(
        json.loads(manifest_path.read_text(encoding="utf-8")), args.variant
    )
    manifest["version"] = args.plugin_version
    output.mkdir(parents=True, exist_ok=True)
    (output / "plugin.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    compatibility = compatibility_manifest(manifest)
    (output / ".codex-plugin/plugin.json").parent.mkdir(parents=True, exist_ok=True)
    (output / ".codex-plugin/plugin.json").write_text(
        json.dumps(compatibility, indent=2) + "\n", encoding="utf-8"
    )
    if args.variant == "local":
        copy_file(ROOT / "adapters/codex/templates/git-bbq-hooks.json", output / "hooks/hooks.json")
    copy_file(ROOT / "adapters/codex/templates/git-bbq-openai.yaml", output / "openai.yaml")
    copy_file(ROOT / "assets/logo.svg", output / "assets/logo.svg")
    copy_file(ROOT / "THIRD_PARTY_NOTICES.md", output / "THIRD_PARTY_NOTICES.md")
    copy_gitbbq_skill(output, args.variant)

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

    if args.variant == "local":
        required_events = {"UserPromptSubmit", "PreToolUse", "PostToolUse", "PostCompact", "Stop"}
        hook_config = json.loads((output / "hooks/hooks.json").read_text(encoding="utf-8"))
        if set(hook_config.get("hooks", {})) != required_events:
            raise SystemExit("local Git BBQ plugin must package exactly the five required lifecycle hooks")
    elif (output / "hooks").exists():
        raise SystemExit("public Git BBQ plugin must not package lifecycle hook files")
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
    parser.add_argument("--variant", choices=["local", "public"], default="local")
    parser.add_argument("--go", default="go")
    parser.add_argument("--force", action="store_true")
    args = parser.parse_args()
    output = build(args)
    print(f"Git BBQ plugin built: {output}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
