package gitbbq

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldRecordsOwnershipAndUninstallPreservesModifiedFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), "example")
	if _, err := ScaffoldProject(root, ScaffoldOptions{
		ProjectName: "Example",
		Problem:     "Track generated ownership safely.",
		Languages:   []string{"go"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, OwnershipFilename)); err != nil {
		t.Fatalf("ownership ledger missing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("user changes\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	assessment, err := AssessUninstall(root)
	if err != nil {
		t.Fatal(err)
	}
	if !assessment.ReadOnly || !containsPath(assessment.Conflicts, "AGENTS.md") {
		t.Fatalf("assessment = %#v", assessment)
	}
	if !containsPath(assessment.Removable, ManifestFilename) {
		t.Fatalf("removable = %#v", assessment.Removable)
	}

	result, err := Uninstall(root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsPath(result.Removed, ManifestFilename) || !containsPath(result.Preserved, "AGENTS.md") {
		t.Fatalf("uninstall result = %#v", result)
	}
	if _, err := os.Stat(filepath.Join(root, ManifestFilename)); !os.IsNotExist(err) {
		t.Fatalf("manifest still exists or returned unexpected error: %v", err)
	}
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil || string(agents) != "user changes\n" {
		t.Fatalf("modified AGENTS.md = %q, err = %v", agents, err)
	}
	if !containsPath(result.Preserved, OwnershipFilename) {
		t.Fatal("ownership ledger was not preserved alongside the conflict")
	}
}

func TestAssessUninstallDoesNotMutateRepository(t *testing.T) {
	root := filepath.Join(t.TempDir(), "example")
	if _, err := ScaffoldProject(root, ScaffoldOptions{ProjectName: "Example", Problem: "Keep cleanup bounded.", Languages: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	before, err := SnapshotFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AssessUninstall(root); err != nil {
		t.Fatal(err)
	}
	after, err := SnapshotFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatal("uninstall assessment changed the repository")
	}
}

func TestUninstallPreservesUnownedDirectories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "example")
	if _, err := ScaffoldProject(root, ScaffoldOptions{ProjectName: "Example", Problem: "Keep directories user-owned.", Languages: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(root); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{filepath.Join(root, ".gitbbq"), filepath.Join(root, ".agents", "skills", "githabits")} {
		if info, err := os.Stat(directory); err != nil || !info.IsDir() {
			t.Fatalf("unowned directory %s was removed: info=%#v err=%v", directory, info, err)
		}
	}
}

func TestUninstallPreservesSymlinkedParentAndOutsideFile(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(outside, "owned.txt")
	content := []byte("outside data\n")
	if err := os.WriteFile(outsideFile, content, 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	ledger := OwnershipLedger{Version: 1, Files: map[string]string{"linked/owned.txt": hex.EncodeToString(digest[:])}}
	data, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".gitbbq"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, OwnershipFilename), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	assessment, err := AssessUninstall(root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsPath(assessment.Conflicts, "linked/owned.txt") || len(assessment.Removable) != 0 {
		t.Fatalf("assessment = %#v", assessment)
	}
	result, err := Uninstall(root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsPath(result.Preserved, "linked/owned.txt") || !containsPath(result.Preserved, OwnershipFilename) {
		t.Fatalf("uninstall result = %#v", result)
	}
	if remaining, err := os.ReadFile(outsideFile); err != nil || string(remaining) != string(content) {
		t.Fatalf("outside file changed: %q, err = %v", remaining, err)
	}
}

func TestOwnershipLedgerRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := OwnershipLedger{Version: 1, Files: map[string]string{"../outside.txt": "deadbeef"}}
	data, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".gitbbq"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, OwnershipFilename), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := AssessUninstall(root); err == nil {
		t.Fatal("unsafe ownership path was accepted")
	}
	if remaining, err := os.ReadFile(outside); err != nil || string(remaining) != "keep\n" {
		t.Fatalf("outside file changed: %q, err = %v", remaining, err)
	}
}
