import tempfile
import unittest
from pathlib import Path

from build_git_bbq_plugin import validate_curation


class CurationValidationTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="git-bbq-curation-test-")
        self.root = Path(self.temporary.name)
        for relative in ("skills/engineering/one", "skills/productivity/two"):
            path = self.root / relative
            path.mkdir(parents=True)
            (path / "SKILL.md").write_text("---\nname: fixture\n---\n", encoding="utf-8")

    def tearDown(self):
        self.temporary.cleanup()

    def manifest(self):
        return {
            "schemaVersion": 2,
            "statuses": ["keep", "hold", "alias", "omit"],
            "derivative": {
                "repository": "https://github.com/averyfreeman/git-bbq-matt-skills.git",
                "release": "v0.2.0",
            },
            "skills": [
                {
                    "path": "skills/engineering/one",
                    "status": "keep",
                    "publicName": "one",
                    "dependencies": [],
                },
                {
                    "path": "skills/productivity/two",
                    "status": "alias",
                    "publicName": "two",
                    "aliasOf": "one",
                    "dependencies": [],
                },
            ],
        }

    def assert_invalid(self, manifest, message):
        with self.assertRaisesRegex(SystemExit, message):
            validate_curation(manifest, self.root)

    def test_accepts_manifest_driven_selection(self):
        selection = validate_curation(self.manifest(), self.root)
        self.assertEqual([(item.source_path, item.public_name) for item in selection], [
            ("skills/engineering/one", "one"),
            ("skills/productivity/two", "two"),
        ])

    def test_rejects_missing_source_paths(self):
        manifest = self.manifest()
        manifest["skills"][0]["path"] = "skills/engineering/missing"
        self.assert_invalid(manifest, "missing source paths")

    def test_rejects_duplicate_public_names(self):
        manifest = self.manifest()
        manifest["skills"][1]["publicName"] = "one"
        self.assert_invalid(manifest, "duplicate public name")

    def test_rejects_invalid_status(self):
        manifest = self.manifest()
        manifest["skills"][0]["status"] = "conditional"
        self.assert_invalid(manifest, "invalid status")

    def test_rejects_alias_cycles(self):
        manifest = self.manifest()
        manifest["skills"][0] = {
            "path": "skills/engineering/one",
            "status": "alias",
            "publicName": "one",
            "aliasOf": "two",
            "dependencies": [],
        }
        self.assert_invalid(manifest, "alias cycle")

    def test_rejects_alias_to_non_packaged_skill(self):
        manifest = self.manifest()
        manifest["skills"][1]["aliasOf"] = "skills/productivity/two"
        manifest["skills"][1]["status"] = "hold"
        manifest["skills"][1].pop("publicName")
        manifest["skills"][1].pop("aliasOf")
        manifest["skills"][0]["aliasOf"] = "skills/productivity/two"
        manifest["skills"][0]["status"] = "alias"
        self.assert_invalid(manifest, "non-packaged skill")


if __name__ == "__main__":
    unittest.main()
