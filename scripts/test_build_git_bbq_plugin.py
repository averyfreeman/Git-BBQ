import json
import tempfile
import unittest
from pathlib import Path

from build_git_bbq_plugin import (
    SKILLS_REPOSITORY,
    CuratedSkill,
    SkillSource,
    compatibility_manifest,
    copy_gitbbq_skill,
    copy_skill_directories,
    export_pinned_selection,
    manifest_for_variant,
    validate_curation,
)
from validate_git_bbq_plugin import validate


COMMIT = "a" * 40


class CurationValidationTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="git-bbq-curation-test-")
        self.root = Path(self.temporary.name)
        for relative in ("skills/engineering/one", "skills/productivity/two"):
            path = self.root / relative
            path.mkdir(parents=True)
            (path / "SKILL.md").write_text("---\nname: fixture\ndescription: fixture\n---\n", encoding="utf-8")

    def tearDown(self):
        self.temporary.cleanup()

    def manifest(self):
        return {
            "schemaVersion": 1,
            "source": {"repository": SKILLS_REPOSITORY, "commit": COMMIT},
            "skills": [
                {"path": "skills/engineering/one", "name": "one"},
                {"path": "skills/productivity/two", "name": "two"},
            ],
        }

    def assert_invalid(self, manifest, message):
        with self.assertRaisesRegex(SystemExit, message):
            validate_curation(manifest, self.root)

    def test_accepts_root_owned_upstream_selection(self):
        selection = validate_curation(self.manifest(), self.root)
        self.assertEqual(
            [(item.source_path, item.name) for item in selection],
            [("skills/engineering/one", "one"), ("skills/productivity/two", "two")],
        )

    def test_rejects_non_upstream_repository(self):
        manifest = self.manifest()
        manifest["source"]["repository"] = "https://github.com/example/skills.git"
        self.assert_invalid(manifest, "must be the upstream repository")

    def test_rejects_unpinned_source(self):
        manifest = self.manifest()
        manifest["source"]["commit"] = "main"
        self.assert_invalid(manifest, "40-character commit")

    def test_rejects_missing_source_paths(self):
        manifest = self.manifest()
        manifest["skills"][0]["path"] = "skills/engineering/missing"
        manifest["skills"][0]["name"] = "missing"
        self.assert_invalid(manifest, "missing SKILL.md")

    def test_rejects_duplicate_names(self):
        manifest = self.manifest()
        manifest["skills"][1]["name"] = "one"
        self.assert_invalid(manifest, "preserve the upstream directory name")

    def test_rejects_renamed_upstream_skill(self):
        manifest = self.manifest()
        manifest["skills"][0]["name"] = "renamed"
        self.assert_invalid(manifest, "preserve the upstream directory name")

    def test_rejects_path_traversal(self):
        manifest = self.manifest()
        manifest["skills"][0]["path"] = "skills/engineering/../one"
        self.assert_invalid(manifest, "invalid upstream path")

    def test_requires_complete_stable_selection(self):
        extra = self.root / "skills/engineering/three"
        extra.mkdir()
        (extra / "SKILL.md").write_text("---\nname: three\ndescription: fixture\n---\n", encoding="utf-8")
        self.assert_invalid(self.manifest(), "unselected stable skills: skills/engineering/three")


class UpstreamSkillPackagingTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="git-bbq-upstream-package-test-")
        self.root = Path(self.temporary.name)
        self.checkout = self.root / "upstream"
        self.skill = self.checkout / "skills/engineering/one"
        (self.skill / "agents").mkdir(parents=True)
        self.source_text = (
            "---\nname: one\ndescription: A sample skill.\n"
            'argument-hint: "What next?"\n'
            "disable-model-invocation: true\n---\n\nOriginal upstream body.\n"
        )
        (self.skill / "SKILL.md").write_text(self.source_text, encoding="utf-8")
        self.openai_text = "policy:\n  allow_implicit_invocation: false\n"
        (self.skill / "agents/openai.yaml").write_text(self.openai_text, encoding="utf-8")
        (self.skill / "REFERENCE.md").write_bytes(b"upstream auxiliary file\n")
        (self.checkout / "LICENSE").write_text("upstream license\n", encoding="utf-8")
        self.selection = (CuratedSkill("skills/engineering/one", "one"),)
        self.source = SkillSource(SKILLS_REPOSITORY, COMMIT, self.checkout, self.selection)
        self.output = self.root / "package"

    def tearDown(self):
        self.temporary.cleanup()

    def test_copies_upstream_skill_under_original_name_and_only_adapts_frontmatter(self):
        copy_skill_directories(self.source, self.output)
        packaged = self.output / "skills/one"
        text = (packaged / "SKILL.md").read_text(encoding="utf-8")
        self.assertIn("name: one", text)
        self.assertNotIn("disable-model-invocation", text)
        self.assertNotIn("\nargument-hint:", text)
        self.assertIn('git-bbq-argument-hint: "What next?"', text)
        self.assertIn("Original upstream body.", text)
        self.assertEqual((packaged / "agents/openai.yaml").read_text(encoding="utf-8"), self.openai_text)
        self.assertEqual((packaged / "REFERENCE.md").read_bytes(), b"upstream auxiliary file\n")
        self.assertEqual(
            (self.output / "licenses/mattpocock-skills/LICENSE").read_text(encoding="utf-8"),
            "upstream license\n",
        )
        manifest = json.loads((self.output / "skills/matt-skills-manifest.json").read_text(encoding="utf-8"))
        self.assertEqual(manifest["skills"][0]["name"], "one")
        self.assertNotIn("aliasOf", manifest["skills"][0])

    def test_rejects_manual_only_field_without_codex_policy(self):
        (self.skill / "agents/openai.yaml").unlink()
        with self.assertRaisesRegex(SystemExit, "lacks Codex invocation policy"):
            copy_skill_directories(self.source, self.output)

    def test_exports_only_committed_upstream_files(self):
        import subprocess

        subprocess.run(["git", "init", str(self.checkout)], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(self.checkout), "config", "user.email", "test@example.com"], check=True)
        subprocess.run(["git", "-C", str(self.checkout), "config", "user.name", "Test"], check=True)
        subprocess.run(["git", "-C", str(self.checkout), "add", "skills/engineering/one", "LICENSE"], check=True)
        subprocess.run(["git", "-C", str(self.checkout), "commit", "-m", "fixture"], check=True, capture_output=True)
        self.source = SkillSource(
            SKILLS_REPOSITORY,
            subprocess.run(
                ["git", "-C", str(self.checkout), "rev-parse", "HEAD"],
                check=True,
                capture_output=True,
                text=True,
            ).stdout.strip(),
            self.checkout,
            self.selection,
        )
        (self.skill / "UNTRACKED.md").write_text("must not be exported\n", encoding="utf-8")
        exported = export_pinned_selection(self.source, self.root / "export")
        copy_skill_directories(exported, self.output)
        packaged = self.output / "skills/one"
        self.assertFalse((packaged / "UNTRACKED.md").exists())
        self.assertEqual((packaged / "REFERENCE.md").read_bytes(), b"upstream auxiliary file\n")
        self.assertEqual(
            (self.output / "licenses/mattpocock-skills/LICENSE").read_text(encoding="utf-8"),
            "upstream license\n",
        )


class PackageValidationTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="git-bbq-package-validation-test-")
        self.package = Path(self.temporary.name)
        (self.package / ".codex-plugin").mkdir()
        (self.package / "hooks").mkdir()
        (self.package / "assets").mkdir()
        (self.package / "runtime/git-bbq").mkdir(parents=True)
        (self.package / "runtime/bin/x86_64-linux").mkdir(parents=True)
        (self.package / "licenses/mattpocock-skills").mkdir(parents=True)
        (self.package / "skills/git-bbq").mkdir(parents=True)
        (self.package / "skills/code-review").mkdir(parents=True)
        (self.package / "skills/setup-matt-pocock-skills").mkdir(parents=True)
        manifest = {
            "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
            "name": "git-bbq",
            "version": "0.4.1",
            "description": "Git BBQ",
            "author": {"name": "Git BBQ"},
            "extensions": {
                "com.openai": {
                    "onboardingSkill": "./skills/setup-matt-pocock-skills/SKILL.md",
                    "hooks": "./hooks/hooks.json",
                    "interface": {
                        "displayName": "Git BBQ",
                        "shortDescription": "Agent workflows for Git repos",
                        "longDescription": "Git BBQ",
                        "developerName": "Git BBQ",
                        "category": "Developer Tools",
                        "defaultPrompt": ["One", "Two", "Three"],
                    },
                }
            },
        }
        (self.package / "plugin.json").write_text(json.dumps(manifest), encoding="utf-8")
        (self.package / ".codex-plugin/plugin.json").write_text(
            json.dumps({
                "name": "git-bbq",
                "author": {"name": "Git BBQ"},
                "interface": {"developerName": "Git BBQ"},
                "extensions": {
                    "com.openai": {
                        "onboardingSkill": "./skills/setup-matt-pocock-skills/SKILL.md"
                    }
                },
            }),
            encoding="utf-8",
        )
        events = {name: [] for name in ("UserPromptSubmit", "PreToolUse", "PostToolUse", "PostCompact", "Stop")}
        (self.package / "hooks/hooks.json").write_text(json.dumps({"hooks": events}), encoding="utf-8")
        (self.package / "assets/logo.svg").write_text("<svg />", encoding="utf-8")
        (self.package / "licenses/mattpocock-skills/LICENSE").write_text("MIT", encoding="utf-8")
        (self.package / "runtime/git-bbq/git-bbq").write_text("#!/bin/sh\n", encoding="utf-8")
        (self.package / "runtime/git-bbq/git-bbq.cmd").write_text("@echo off\n", encoding="utf-8")
        (self.package / "runtime/bin/x86_64-linux/git-bbq").write_text("binary", encoding="utf-8")
        (self.package / "skills/git-bbq/SKILL.md").write_text(
            "---\nname: git-bbq\ndescription: Git BBQ repository workflow.\n---\n", encoding="utf-8"
        )
        (self.package / "skills/code-review/SKILL.md").write_text(
            "---\nname: code-review\ndescription: Review code changes.\n---\n", encoding="utf-8"
        )
        (self.package / "skills/setup-matt-pocock-skills/SKILL.md").write_text(
            "---\nname: setup-matt-pocock-skills\ndescription: Configure repository workflows.\n---\n",
            encoding="utf-8",
        )
        commit = COMMIT
        records = [
            {
                "sourcePath": "skills/engineering/code-review",
                "name": "code-review",
                "repository": SKILLS_REPOSITORY,
                "commit": commit,
            },
            {
                "sourcePath": "skills/engineering/setup-matt-pocock-skills",
                "name": "setup-matt-pocock-skills",
                "repository": SKILLS_REPOSITORY,
                "commit": commit,
            },
        ]
        (self.package / "skills/matt-skills-manifest.json").write_text(
            json.dumps({"repository": SKILLS_REPOSITORY, "commit": commit, "skills": records}),
            encoding="utf-8",
        )

    def tearDown(self):
        self.temporary.cleanup()

    def test_accepts_spec_valid_package_and_names(self):
        self.assertEqual(validate(self.package, "x86_64-linux"), [])

    def test_rejects_name_directory_mismatch(self):
        skill_path = self.package / "skills/code-review/SKILL.md"
        skill_path.write_text("---\nname: review-code\ndescription: Review code changes.\n---\n", encoding="utf-8")
        errors = validate(self.package, "x86_64-linux")
        self.assertTrue(any("must match its parent directory" in error for error in errors))

    def test_rejects_nonstandard_frontmatter(self):
        skill_path = self.package / "skills/code-review/SKILL.md"
        skill_path.write_text(
            "---\nname: code-review\ndescription: Review code changes.\ndisable-model-invocation: true\n---\n",
            encoding="utf-8",
        )
        errors = validate(self.package, "x86_64-linux")
        self.assertTrue(any("nonstandard frontmatter field" in error for error in errors))

    def test_rejects_short_description_over_limit(self):
        manifest_path = self.package / "plugin.json"
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        manifest["extensions"]["com.openai"]["interface"]["shortDescription"] = "A description that is much too long for the UI"
        manifest_path.write_text(json.dumps(manifest), encoding="utf-8")
        errors = validate(self.package, "x86_64-linux")
        self.assertTrue(any("shortDescription exceeds 30" in error for error in errors))

    def test_rejects_onboarding_path_missing_from_compatibility_manifest(self):
        manifest_path = self.package / ".codex-plugin/plugin.json"
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        del manifest["extensions"]["com.openai"]["onboardingSkill"]
        manifest_path.write_text(json.dumps(manifest), encoding="utf-8")
        errors = validate(self.package, "x86_64-linux")
        self.assertTrue(any("same onboardingSkill" in error for error in errors))

    def test_rejects_onboarding_path_that_does_not_resolve(self):
        (self.package / "skills/setup-matt-pocock-skills/SKILL.md").unlink()
        errors = validate(self.package, "x86_64-linux")
        self.assertTrue(any("does not resolve to a packaged skill" in error for error in errors))

    def test_rejects_onboarding_path_that_does_not_match_setup_skill(self):
        manifest_path = self.package / "plugin.json"
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        manifest["extensions"]["com.openai"]["onboardingSkill"] = "./skills/missing/SKILL.md"
        manifest_path.write_text(json.dumps(manifest), encoding="utf-8")
        errors = validate(self.package, "x86_64-linux")
        self.assertTrue(any("must point to the bundled setup" in error for error in errors))

    def test_reports_malformed_hook_configuration_without_crashing(self):
        (self.package / "hooks/hooks.json").write_text(json.dumps({"hooks": []}), encoding="utf-8")
        errors = validate(self.package, "x86_64-linux")
        self.assertTrue(any("hooks must be an object" in error for error in errors))

    def test_reports_malformed_nested_manifests_without_crashing(self):
        for relative in ("plugin.json", ".codex-plugin/plugin.json"):
            path = self.package / relative
            manifest = json.loads(path.read_text(encoding="utf-8"))
            manifest["extensions"] = []
            path.write_text(json.dumps(manifest), encoding="utf-8")

        errors = validate(self.package, "x86_64-linux")
        self.assertGreaterEqual(sum("must be an object" in error for error in errors), 2)


class CompatibilityManifestTests(unittest.TestCase):
    def test_preserves_only_onboarding_from_openai_extension(self):
        portable = {
            "name": "git-bbq",
            "extensions": {
                "com.openai": {
                    "onboardingSkill": "./skills/setup-matt-pocock-skills/SKILL.md",
                    "hooks": "./hooks/hooks.json",
                    "interface": {"displayName": "Git BBQ"},
                }
            },
        }
        compatibility = compatibility_manifest(portable)
        self.assertEqual(
            compatibility["extensions"]["com.openai"],
            {"onboardingSkill": "./skills/setup-matt-pocock-skills/SKILL.md"},
        )
        self.assertNotIn("hooks", compatibility["extensions"]["com.openai"])


class VariantManifestTests(unittest.TestCase):
    def test_local_variant_keeps_hook_reference(self):
        portable = {
            "name": "git-bbq",
            "extensions": {"com.openai": {"hooks": "./hooks/hooks.json", "interface": {}}},
        }
        local = manifest_for_variant(portable, "local")
        self.assertEqual(local["extensions"]["com.openai"]["hooks"], "./hooks/hooks.json")
        self.assertEqual(portable["extensions"]["com.openai"]["hooks"], "./hooks/hooks.json")

    def test_public_variant_omits_hook_reference_and_describes_local_runtime(self):
        portable = {
            "name": "git-bbq",
            "extensions": {
                "com.openai": {
                    "hooks": "./hooks/hooks.json",
                    "interface": {"longDescription": "Includes lifecycle hooks."},
                }
            },
        }
        public = manifest_for_variant(portable, "public")
        self.assertNotIn("hooks", public["extensions"]["com.openai"])
        description = public["extensions"]["com.openai"]["interface"]["longDescription"]
        self.assertIn("local Codex CLI", description)
        self.assertIn("hooks", portable["extensions"]["com.openai"])

    def test_rejects_unknown_variant(self):
        with self.assertRaisesRegex(SystemExit, "choose local or public"):
            manifest_for_variant({}, "cloud")

    def test_public_git_bbq_skill_omits_local_hook_setup(self):
        with tempfile.TemporaryDirectory(prefix="git-bbq-skill-variant-test-") as temporary:
            output = Path(temporary)
            copy_gitbbq_skill(output, "public")
            skill = (output / "skills/git-bbq/SKILL.md").read_text(encoding="utf-8")
            self.assertNotIn("<!-- local-hooks:", skill)
            self.assertNotIn("git-bbq help hooks", skill)
            self.assertNotIn("/hooks", skill)

            copy_gitbbq_skill(output, "local")
            local_skill = (output / "skills/git-bbq/SKILL.md").read_text(encoding="utf-8")
            self.assertIn("git-bbq help hooks", local_skill)
            self.assertNotIn("<!-- local-hooks:", local_skill)


class PluginVariantValidationTests(unittest.TestCase):
    def setUp(self):
        self.package_tests = PackageValidationTests()
        self.package_tests.setUp()
        self.package = self.package_tests.package

    def tearDown(self):
        self.package_tests.tearDown()

    def test_accepts_hook_free_public_variant_with_runtime_and_skills(self):
        manifest_path = self.package / "plugin.json"
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        del manifest["extensions"]["com.openai"]["hooks"]
        manifest["extensions"]["com.openai"]["interface"]["longDescription"] = (
            "Git BBQ works through a local Codex CLI."
        )
        manifest_path.write_text(json.dumps(manifest), encoding="utf-8")
        import shutil

        shutil.rmtree(self.package / "hooks")

        self.assertEqual(validate(self.package, "x86_64-linux", variant="public"), [])
        self.assertTrue((self.package / "runtime/git-bbq/git-bbq").is_file())
        self.assertTrue((self.package / "skills/code-review/SKILL.md").is_file())

    def test_public_variant_rejects_hook_files_and_manifest_references(self):
        errors = validate(self.package, "x86_64-linux", variant="public")
        self.assertTrue(any("must not reference lifecycle hooks" in error for error in errors))
        self.assertTrue(any("must not contain hook files" in error for error in errors))

    def test_public_variant_rejects_hook_reference_in_compatibility_manifest(self):
        path = self.package / ".codex-plugin/plugin.json"
        manifest = json.loads(path.read_text(encoding="utf-8"))
        manifest["extensions"]["com.openai"]["hooks"] = "./hooks/hooks.json"
        path.write_text(json.dumps(manifest), encoding="utf-8")
        errors = validate(self.package, "x86_64-linux", variant="public")
        self.assertTrue(any("public manifest contains a hook reference" in error for error in errors))


if __name__ == "__main__":
    unittest.main()
