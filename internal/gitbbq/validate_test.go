package gitbbq

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateProjectDetectsProjectionDriftAndProjectRepairsIt(t *testing.T) {
	root := filepath.Join(t.TempDir(), "example")
	if _, err := ScaffoldProject(root, ScaffoldOptions{ProjectName: "Example", Problem: "Keep project records consistent.", Languages: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateADR(root, ADRInput{Title: "Use stable projections", Context: "Consumers need one current view.", Decision: "Regenerate projections from canonical records.", Why: "It avoids duplicate decision history.", Status: "accepted"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Project(root); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProject(root); err != nil {
		t.Fatalf("fresh projections did not validate: %v", err)
	}

	manifest, err := ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Problem = "A revised problem statement changes the projections."
	data, err := marshalYAML(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ManifestFilename), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProject(root); err == nil || !strings.Contains(err.Error(), "out of date") {
		t.Fatalf("drifted projections validation error = %v", err)
	}
	if _, err := Project(root); err != nil {
		t.Fatalf("project did not repair drift: %v", err)
	}
	if err := ValidateProject(root); err != nil {
		t.Fatalf("repaired projections did not validate: %v", err)
	}
}

func TestValidateProjectIgnoresVolatileADRIndexTimestamp(t *testing.T) {
	root := filepath.Join(t.TempDir(), "example")
	if _, err := ScaffoldProject(root, ScaffoldOptions{ProjectName: "Example", Problem: "Keep index timestamps volatile.", Languages: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateADR(root, ADRInput{Title: "Index a decision", Context: "Agents need a retrieval view.", Decision: "Generate an ADR index.", Why: "It keeps lookup straightforward.", Status: "accepted"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Project(root); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, ADRIndexFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var index ADRIndex
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatal(err)
	}
	index.GeneratedAt = index.GeneratedAt.Add(24 * time.Hour)
	data, err = json.MarshalIndent(index, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProject(root); err != nil {
		t.Fatalf("timestamp-only index change was treated as drift: %v", err)
	}
}

func TestValidateProjectReportsMissingProjectionWithoutRepairingIt(t *testing.T) {
	for _, relative := range []string{ContractFilename, ImplementationPlanFilename, ADRIndexFilename} {
		t.Run(relative, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "example")
			if _, err := ScaffoldProject(root, ScaffoldOptions{ProjectName: "Example", Problem: "Require a complete projection set.", Languages: []string{"go"}}); err != nil {
				t.Fatal(err)
			}
			if _, err := CreateADR(root, ADRInput{Title: "Keep projection set complete", Context: "Consumers need current generated views.", Decision: "Generate every required projection together.", Why: "Partial derived state hides missing updates.", Status: "accepted"}); err != nil {
				t.Fatal(err)
			}
			if _, err := Project(root); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, filepath.FromSlash(relative))
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			before, err := SnapshotFiles(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateProject(root); err == nil || !strings.Contains(err.Error(), "missing "+relative) {
				t.Fatalf("missing projection error = %v", err)
			}
			after, err := SnapshotFiles(root)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(before, "\n") != strings.Join(after, "\n") {
				t.Fatal("validation changed the missing projection set")
			}
			if _, err := Project(root); err != nil {
				t.Fatalf("project did not repair %s: %v", relative, err)
			}
			if err := ValidateProject(root); err != nil {
				t.Fatalf("repaired projection set did not validate: %v", err)
			}
		})
	}
}

func TestValidateProjectReportsMalformedProjection(t *testing.T) {
	root := filepath.Join(t.TempDir(), "example")
	if _, err := ScaffoldProject(root, ScaffoldOptions{ProjectName: "Example", Problem: "Report malformed projections.", Languages: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Project(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ContractFilename)
	if err := os.WriteFile(path, []byte("schema_version: [broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProject(root); err == nil || !strings.Contains(err.Error(), "read "+ContractFilename+" projection") {
		t.Fatalf("malformed projection error = %v", err)
	}
}

func TestValidateProjectRedactsSecretFindingValues(t *testing.T) {
	root := filepath.Join(t.TempDir(), "example")
	if _, err := ScaffoldProject(root, ScaffoldOptions{ProjectName: "Example", Problem: "Reject committed credentials.", Languages: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	secret := "0123456789abcdef_PRIVATE_SECRET"
	if err := os.WriteFile(filepath.Join(root, ContextFilename), []byte("# Example\n\napi_key: \""+secret+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateProject(root)
	if err == nil || !strings.Contains(err.Error(), `"CONTEXT.md":3`) || !strings.Contains(err.Error(), "credential assignment") {
		t.Fatalf("secret finding error = %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("secret value leaked in diagnostic: %v", err)
	}
}

func TestProjectCanRepairSecretBearingGeneratedProjection(t *testing.T) {
	root := filepath.Join(t.TempDir(), "example")
	if _, err := ScaffoldProject(root, ScaffoldOptions{ProjectName: "Example", Problem: "Regenerate derived views.", Languages: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Project(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ImplementationPlanFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	secret := "0123456789abcdef_PRIVATE_SECRET"
	data = append(data, []byte("\napi_key: \""+secret+"\"\n")...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProject(root); err == nil || !strings.Contains(err.Error(), "secret-like values") {
		t.Fatalf("generated secret validation error = %v", err)
	}
	if _, err := Project(root); err != nil {
		t.Fatalf("project failed to regenerate derived output: %v", err)
	}
	if err := ValidateProject(root); err != nil {
		t.Fatalf("regenerated outputs did not validate: %v", err)
	}
}

func TestValidateProjectBoundsArchitectureArtifactScanning(t *testing.T) {
	root := filepath.Join(t.TempDir(), "example")
	if _, err := ScaffoldProject(root, ScaffoldOptions{ProjectName: "Example", Problem: "Bound validation input.", Languages: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	large := make([]byte, maxArchitectureArtifactBytes+1)
	for i := range large {
		large[i] = 'x'
	}
	if err := os.WriteFile(filepath.Join(root, ContextFilename), large, 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateProject(root)
	if err == nil || !strings.Contains(err.Error(), "per-file limit") || strings.Contains(err.Error(), "xxxx") {
		t.Fatalf("bounded scan error = %v", err)
	}
}

func TestADRIndexPathsAreRepositoryRelative(t *testing.T) {
	root := filepath.Join(t.TempDir(), "example")
	if _, err := ScaffoldProject(root, ScaffoldOptions{ProjectName: "Example", Problem: "Keep retrieval indexes portable.", Languages: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateADR(root, ADRInput{Title: "Use relative index paths", Context: "Repositories can move.", Decision: "Store repository-relative ADR paths.", Why: "Absolute paths belong to one machine.", Status: "accepted"}); err != nil {
		t.Fatal(err)
	}
	index, err := IndexADRs(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := index.Decisions[0].Path, "docs/adr/0001-use-relative-index-paths.md"; got != want {
		t.Fatalf("ADR index path = %q, want %q", got, want)
	}
}
