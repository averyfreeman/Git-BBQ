package gitbbq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssessMigrationIsReadOnlyAndMapsLegacyFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".adr-scaffold.yaml"), []byte("version: 1\nlanguage: go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".ai-architect", "decisions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ai-architect", "project-context.md"), []byte("---\nlanguage: go\nmotivation:\n  problem: Keep decisions durable.\n---\n\n# Context\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before, err := SnapshotFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	assessment, err := AssessMigration(root)
	if err != nil {
		t.Fatal(err)
	}
	if !assessment.ReadOnly || !assessment.Detected {
		t.Fatalf("assessment = %#v", assessment)
	}
	if !containsPath(assessment.Legacy, ".adr-scaffold.yaml") || !containsPath(assessment.Legacy, ".ai-architect/project-context.md") {
		t.Fatalf("legacy paths = %#v", assessment.Legacy)
	}
	if !containsPath(assessment.Proposed, ManifestFilename) || !containsPath(assessment.Proposed, ContextFilename) {
		t.Fatalf("proposed paths = %#v", assessment.Proposed)
	}
	if !strings.Contains(strings.Join(assessment.Warnings, "\n"), "approval") {
		t.Fatalf("warnings = %#v", assessment.Warnings)
	}
	after, err := SnapshotFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatal("migration assessment changed the repository")
	}
}

func TestAssessMigrationReportsMissingLegacyScaffold(t *testing.T) {
	assessment, err := AssessMigration(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if assessment.Detected || len(assessment.Legacy) != 0 {
		t.Fatalf("unexpected legacy detection: %#v", assessment)
	}
}

func TestMigrateConvertsValidatedADRsAndPreservesLegacyFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".adr-scaffold.yaml"), []byte("version: 1\nlanguage: go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, GitHabitsFilename), []byte("version: [invalid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".ai-architect", "decisions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ai-architect", "project-context.md"), []byte("---\nlanguage: go\nmotivation:\n  problem: Important decisions otherwise disappear.\n---\n\n# Context\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyContent := "---\nschema_version: 1.0.0\nrevision: 1\ndecision:\n  id: ADR-001\n  title: Use a local ADR record\n  status: accepted\n  context: Important decisions otherwise disappear.\n  motivation:\n    why_this_solution: A local record is reviewable beside the code.\n  decision: Keep architecture decisions in local records.\n---\n\n# Decision\n"
	if err := os.WriteFile(filepath.Join(root, ".ai-architect", "decisions", "ADR-001-use-a-local-adr-record.md"), []byte(legacyContent), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyADR, err := filepath.Glob(filepath.Join(root, ".ai-architect", "decisions", "ADR-001-*.md"))
	if err != nil || len(legacyADR) != 1 {
		t.Fatalf("legacy ADRs = %#v, err = %v", legacyADR, err)
	}

	result, err := Migrate(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.MigratedADRs != 1 {
		t.Fatalf("migration result = %#v", result)
	}
	if _, err := os.Stat(legacyADR[0]); err != nil {
		t.Fatalf("legacy ADR was not preserved: %v", err)
	}
	targetADR, err := filepath.Glob(filepath.Join(root, ADRDirectory, "0001-use-a-local-adr-record.md"))
	if err != nil || len(targetADR) != 1 {
		t.Fatalf("target ADRs = %#v, err = %v", targetADR, err)
	}
	if _, err := ParseADR(targetADR[0]); err != nil {
		t.Fatalf("migrated ADR is invalid: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".gitbbq", "migration", "legacy", GitHabitsFilename)); err != nil {
		t.Fatalf("legacy Git habits archive missing: %v", err)
	}
	for _, required := range []string{ADRIndexFilename, ContractFilename, ImplementationPlanFilename, GitHabitsFilename, filepath.ToSlash(filepath.Join(".gitbbq", "migration", "legacy", GitHabitsFilename))} {
		if !containsPath(result.Created, required) {
			t.Fatalf("migration did not report created artifact %q: %#v", required, result.Created)
		}
	}
	ledger, err := readOwnershipLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, owned := range []string{targetADR[0], filepath.Join(root, ADRIndexFilename), filepath.Join(root, ContractFilename), filepath.Join(root, ImplementationPlanFilename), filepath.Join(root, GitHabitsFilename), filepath.Join(root, ".gitbbq", "migration", "legacy", GitHabitsFilename)} {
		relative := relativePath(root, owned)
		if ledger.Files[relative] == "" {
			t.Fatalf("migrated artifact is not owned: %s", relative)
		}
	}
	assessment, err := AssessUninstall(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(assessment.Conflicts) != 0 {
		t.Fatalf("migrated ownership conflicts = %#v", assessment.Conflicts)
	}
	if err := ValidateProject(root); err != nil {
		t.Fatalf("migrated project is invalid: %v", err)
	}
}

func containsPath(paths []string, expected string) bool {
	for _, path := range paths {
		if path == expected {
			return true
		}
	}
	return false
}
