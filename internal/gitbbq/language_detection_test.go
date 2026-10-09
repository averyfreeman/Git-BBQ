package gitbbq

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDetectLanguagesFindsNestedPolyglotSourcesAndSkipsGeneratedTrees(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                          "module example\n",
		"cmd/server/main.go":              "package main\n",
		"services/worker/task.py":         "def run(): pass\n",
		"web/client/src/app.ts":           "export const app = true\n",
		"web/client/src/legacy.js":        "export const legacy = true\n",
		"web/client/package.json":         "{}\n",
		"web/client/tsconfig.json":        "{}\n",
		"apps/legacy/package.json":        "{}\n",
		"vendor/dependency/dependency.rs": "fn main() {}\n",
		"node_modules/pkg/index.js":       "module.exports = {}\n",
		"build/generated/Thing.java":      "class Thing {}\n",
		".agents/skills/sample/SKILL.md":  "# Skill\n",
	}
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := DetectLanguages(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"go": true, "python": true, "typescript": true, "javascript": true}
	for _, candidate := range result.Candidates {
		if !want[candidate.Language] {
			t.Errorf("unexpected candidate %q", candidate.Language)
		}
		delete(want, candidate.Language)
	}
	if len(want) != 0 {
		t.Errorf("missing candidates: %#v", want)
	}
	for _, candidate := range result.Candidates {
		for _, evidence := range candidate.Evidence {
			if evidence.Path == "vendor/dependency/dependency.rs" || evidence.Path == "node_modules/pkg/index.js" || evidence.Path == "build/generated/Thing.java" {
				t.Errorf("excluded tree contributed evidence: %#v", evidence)
			}
		}
	}
	for _, candidate := range result.Candidates {
		if candidate.Language != "javascript" {
			continue
		}
		for _, evidence := range candidate.Evidence {
			if evidence.Path == "apps/legacy/package.json" && evidence.Kind == "manifest" {
				return
			}
		}
	}
	t.Fatal("JavaScript package manifest was lost when a nested TypeScript package was detected")
}

func TestDetectLanguagesUsesJavaScriptForPackageManifestOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"test":"node test.js"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := DetectLanguages(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.CandidateLanguages(); len(got) != 1 || got[0] != "javascript" {
		t.Fatalf("candidates = %#v, want [javascript]", got)
	}
}

func TestDetectLanguagesReturnsNoCandidatesForEmptyRepository(t *testing.T) {
	result, err := DetectLanguages(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("empty repository candidates = %#v", result.Candidates)
	}
}

func TestDetectLanguagesCapsEvidenceWithoutLosingCandidate(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < maxLanguageEvidence+3; i++ {
		path := filepath.Join(root, "src", "file-"+strconv.Itoa(i)+".go")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package src\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := DetectLanguages(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Language != "go" {
		t.Fatalf("candidates = %#v", result.Candidates)
	}
	if got := result.Candidates[0].EvidenceOmitted; got != 3 {
		t.Fatalf("omitted evidence = %d, want 3", got)
	}
}
