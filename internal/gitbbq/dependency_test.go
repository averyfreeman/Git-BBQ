package gitbbq

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultManifestPinsUpstreamMattSkills(t *testing.T) {
	manifest := DefaultManifest("Example")
	if manifest.Matt.Repository != MattRepository || manifest.Matt.Commit != MattCommit || manifest.Matt.Path != MattDependencyPath {
		t.Fatalf("manifest Matt dependency = %#v", manifest.Matt)
	}
}

func TestUpdateMattDependencyKeepsProjectPathAndMetadataInSync(t *testing.T) {
	root := filepath.Join(t.TempDir(), "example")
	if _, err := ScaffoldProject(root, ScaffoldOptions{ProjectName: "Example", Problem: "Pin skills safely.", Languages: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	updated, err := UpdateMattDependency(root, MattDependency{
		Repository: MattRepository,
		Commit:     MattCommit,
		Path:       MattDependencyPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Matt.Repository != MattRepository || updated.Matt.Commit != MattCommit {
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

func TestUpdateMattDependencyRejectsPathDrift(t *testing.T) {
	root := filepath.Join(t.TempDir(), "example")
	if _, err := ScaffoldProject(root, ScaffoldOptions{ProjectName: "Example", Problem: "Pin skills safely.", Languages: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateMattDependency(root, MattDependency{
		Repository: MattRepository,
		Commit:     MattCommit,
		Path:       ".agents/other-skills",
	}); err == nil {
		t.Fatal("expected dependency path drift to be rejected")
	}
}
