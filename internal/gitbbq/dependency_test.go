package gitbbq

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateMattDependencyKeepsProjectPathAndMetadataInSync(t *testing.T) {
	root := filepath.Join(t.TempDir(), "example")
	if _, err := ScaffoldProject(root, ScaffoldOptions{ProjectName: "Example", Problem: "Pin skills safely.", Languages: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	updated, err := UpdateMattDependency(root, MattDependency{
		Repository: "https://github.com/example/skills.git",
		Commit:     "0123456789abcdef",
		Path:       MattDependencyPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Matt.Commit != "0123456789abcdef" {
		t.Fatalf("manifest = %#v", updated.Matt)
	}
	if err := ValidateProject(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, MattDependencyMetadataPath)); err != nil {
		t.Fatal(err)
	}
	assessment, err := AssessUninstall(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{ManifestFilename, MattDependencyMetadataPath} {
		if containsPath(assessment.Conflicts, relative) {
			t.Fatalf("updated dependency artifact remained a conflict: %#v", assessment.Conflicts)
		}
	}
}
