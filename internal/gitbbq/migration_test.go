package gitbbq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssessMigrationIsReadOnlyAndReportsConversion(t *testing.T) {
	root := t.TempDir()
	writeLegacyMigrationFixture(t, root, true, true)
	before := readFixtureFiles(t, root,
		LegacyScaffoldFilename,
		filepath.ToSlash(filepath.Join(LegacyArchitectDir, "project-context.md")),
		filepath.ToSlash(filepath.Join(LegacyArchitectDir, "decisions", "ADR-001-use-a-local-adr.md")),
		GitHabitsFilename,
	)

	assessment, err := AssessMigration(root)
	if err != nil {
		t.Fatal(err)
	}
	if !assessment.ReadOnly || !assessment.Detected || !assessment.Compatible {
		t.Fatalf("assessment = %#v", assessment)
	}
	if assessment.MigratedADRs != 1 || len(assessment.RequiredApprovals) != 1 {
		t.Fatalf("assessment omitted converted ADR or archive approval: %#v", assessment)
	}
	if !containsPath(assessment.Legacy, LegacyScaffoldFilename) || !containsPath(assessment.Legacy, LegacyArchitectDir) {
		t.Fatalf("legacy inventory = %#v", assessment.Legacy)
	}
	if !containsPath(assessment.Proposed, ManifestFilename) || !containsPath(assessment.Proposed, ContextFilename) ||
		!containsPath(assessment.Proposed, "docs/adr/0001-use-a-local-adr.md") {
		t.Fatalf("proposed files = %#v", assessment.Proposed)
	}
	after := readFixtureFiles(t, root,
		LegacyScaffoldFilename,
		filepath.ToSlash(filepath.Join(LegacyArchitectDir, "project-context.md")),
		filepath.ToSlash(filepath.Join(LegacyArchitectDir, "decisions", "ADR-001-use-a-local-adr.md")),
		GitHabitsFilename,
	)
	for path, content := range before {
		if string(after[path]) != string(content) {
			t.Fatalf("assessment changed legacy source %s", path)
		}
	}
}

func TestMigrateRequiresSeparateArchiveApprovalAndPreservesLegacySources(t *testing.T) {
	root := t.TempDir()
	writeLegacyMigrationFixture(t, root, true, true)
	legacyDecisionPath := filepath.Join(root, LegacyArchitectDir, "decisions", "ADR-001-use-a-local-adr.md")
	legacyDecision, err := os.ReadFile(legacyDecisionPath)
	if err != nil {
		t.Fatal(err)
	}
	legacyHabits, err := os.ReadFile(filepath.Join(root, GitHabitsFilename))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Migrate(root, false); err == nil || !strings.Contains(err.Error(), "--archive-legacy-githabits") {
		t.Fatalf("migration without archive approval error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ManifestFilename)); !os.IsNotExist(err) {
		t.Fatalf("unapproved migration wrote the manifest: %v", err)
	}

	result, err := Migrate(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.MigratedADRs != 1 {
		t.Fatalf("migration result = %#v", result)
	}
	if content, err := os.ReadFile(legacyDecisionPath); err != nil || string(content) != string(legacyDecision) {
		t.Fatalf("legacy decision was modified: err=%v", err)
	}
	if content, err := os.ReadFile(filepath.Join(root, GitHabitsFilename)); err != nil || strings.Contains(string(content), "legacy-profile") {
		t.Fatalf("incompatible Git habits file was not replaced: err=%v content=%s", err, content)
	}
	archivePath := filepath.Join(root, ".gitbbq", "migration", "legacy", GitHabitsFilename)
	if content, err := os.ReadFile(archivePath); err != nil || string(content) != string(legacyHabits) {
		t.Fatalf("legacy Git habits archive mismatch: err=%v", err)
	}
	if !containsPath(result.Archived, filepath.ToSlash(filepath.Join(".gitbbq", "migration", "legacy", GitHabitsFilename))) {
		t.Fatalf("migration result omitted archive: %#v", result)
	}
	adrPath := filepath.Join(root, "docs", "adr", "0001-use-a-local-adr.md")
	adr, err := ParseADR(adrPath)
	if err != nil {
		t.Fatalf("migrated ADR is invalid: %v", err)
	}
	if adr.Status != "accepted" || !strings.Contains(adr.Body, "We decided: Use a local record.") {
		t.Fatalf("migrated ADR did not retain validated decision fields: %#v", adr)
	}
	if err := ValidateProject(root); err != nil {
		t.Fatalf("migrated project failed validation: %v", err)
	}
	ledger, err := readOwnershipLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Files[filepath.ToSlash(filepath.Join(ADRDirectory, "0001-use-a-local-adr.md"))] == "" {
		t.Fatal("migrated ADR is missing from the Git BBQ ownership ledger")
	}
	if _, err := os.Stat(filepath.Join(root, LegacyScaffoldFilename)); err != nil {
		t.Fatalf("legacy scaffold metadata was removed: %v", err)
	}
	before, err := SnapshotFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Migrate(root, true); err == nil || !strings.Contains(err.Error(), ManifestFilename) {
		t.Fatalf("repeated migration error = %v", err)
	}
	after, err := SnapshotFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatal("rejected repeated migration changed project files")
	}
}

func TestAssessMigrationBlocksExistingManifestWithoutWriting(t *testing.T) {
	root := t.TempDir()
	writeLegacyMigrationFixture(t, root, false, false)
	manifestPath := filepath.Join(root, ManifestFilename)
	if err := os.WriteFile(manifestPath, []byte("keep this user file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	assessment, err := AssessMigration(root)
	if err != nil {
		t.Fatal(err)
	}
	if assessment.Compatible || !strings.Contains(strings.Join(assessment.Blockers, "\n"), ManifestFilename) {
		t.Fatalf("assessment did not block manifest conflict: %#v", assessment)
	}
	if _, err := Migrate(root, false); err == nil {
		t.Fatal("migration proceeded despite an existing manifest")
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil || string(data) != "keep this user file\n" {
		t.Fatalf("existing manifest changed: err=%v content=%s", err, data)
	}
	if _, err := os.Stat(filepath.Join(root, GitHabitsFilename)); !os.IsNotExist(err) {
		t.Fatalf("blocked migration wrote Git habits: %v", err)
	}
}

func TestAssessMigrationRejectsSecretWithoutLeakingValue(t *testing.T) {
	root := t.TempDir()
	writeLegacyMigrationFixture(t, root, false, false)
	secret := "0123456789abcdef_PRIVATE_SECRET"
	contextPath := filepath.Join(root, LegacyArchitectDir, "project-context.md")
	if err := os.WriteFile(contextPath, []byte("---\nschema_version: 1.1.0\nlanguage: go\nmotivation:\n  problem: \"api_key: "+secret+"\"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	assessment, err := AssessMigration(root)
	if err != nil {
		t.Fatal(err)
	}
	blockers := strings.Join(assessment.Blockers, "\n")
	if assessment.Compatible || !strings.Contains(blockers, "secret-like value") || strings.Contains(blockers, secret) {
		t.Fatalf("secret handling was not redacted: %#v", assessment)
	}
}

func TestAssessMigrationBlocksUnsafeLegacySymlink(t *testing.T) {
	root := t.TempDir()
	writeLegacyMigrationFixture(t, root, false, false)
	decisionPath := filepath.Join(root, LegacyArchitectDir, "decisions", "ADR-001-use-a-local-adr.md")
	if err := os.MkdirAll(filepath.Dir(decisionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "decision.md")
	if err := os.WriteFile(target, []byte("not a migration source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, decisionPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	assessment, err := AssessMigration(root)
	if err != nil {
		t.Fatal(err)
	}
	if assessment.Compatible || !strings.Contains(strings.Join(assessment.Blockers, "\n"), "bounded regular file") {
		t.Fatalf("assessment accepted legacy symlink: %#v", assessment)
	}
}

func TestAssessMigrationBlocksMalformedLegacyDecisionWithoutWriting(t *testing.T) {
	root := t.TempDir()
	writeLegacyMigrationFixture(t, root, true, false)
	decisionPath := filepath.Join(root, LegacyArchitectDir, "decisions", "ADR-001-use-a-local-adr.md")
	data, err := os.ReadFile(decisionPath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "id: ADR-001", "id: ADR-002", 1))
	if err := os.WriteFile(decisionPath, data, 0o644); err != nil {
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
	if assessment.Compatible || !strings.Contains(strings.Join(assessment.Blockers, "\n"), "id and filename do not match") {
		t.Fatalf("malformed decision was not blocked: %#v", assessment)
	}
	if _, err := Migrate(root, false); err == nil {
		t.Fatal("migration accepted a malformed legacy decision")
	}
	after, err := SnapshotFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatal("rejected malformed migration changed project files")
	}
}

func writeLegacyMigrationFixture(t *testing.T, root string, includeDecision, includeGitHabits bool) {
	t.Helper()
	metadata := `version: 1
language: go
features:
  agents: true
  skills: true
  claude: false
  adr: true
  context: true
  contract: true
  implementation_plan: true
  pages: false
  memory: false
paths:
  architect: .ai-architect
  decisions: .ai-architect/decisions
  docs_site: docs/site
`
	if err := os.WriteFile(filepath.Join(root, LegacyScaffoldFilename), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, LegacyArchitectDir), 0o755); err != nil {
		t.Fatal(err)
	}
	context := `---
schema_version: 1.1.0
language: go
motivation:
  why_building: Preserve architecture intent.
  problem: Keep decisions durable.
  audience: Maintainers.
  alternatives: Chat history.
  why_this_solution: Versioned repository records.
---
# Project context
`
	if err := os.WriteFile(filepath.Join(root, LegacyArchitectDir, "project-context.md"), []byte(context), 0o644); err != nil {
		t.Fatal(err)
	}
	if includeDecision {
		decisionDirectory := filepath.Join(root, LegacyArchitectDir, "decisions")
		if err := os.MkdirAll(decisionDirectory, 0o755); err != nil {
			t.Fatal(err)
		}
		decision := `---
schema_version: 1.0.0
revision: 1
decision:
  id: ADR-001
  title: Use a local ADR
  date: 2026-10-08
  status: accepted
  motivation:
    why_building: Preserve architecture intent.
    problem: Keep decisions durable.
    audience: Maintainers.
    alternatives: Chat history.
    why_this_solution: Versioned repository records.
  context: The project needs durable decisions.
  drivers: [Reviewability]
  considered_option_ids: [OPT-001, OPT-002]
  selected_option_id: OPT-001
  decision: Use a local record.
  positive_consequences: [Changes stay reviewable.]
  negative_consequences: [Records require maintenance.]
  assumptions: [The repository is version controlled.]
  validation_criteria: [The record parses.]
  supersedes: []
  decision_matrix: []
  action_contract:
    good_outcomes: []
    bad_outcomes: []
    correct_example: ''
    incorrect_example: ''
---
# Legacy decision
`
		if err := os.WriteFile(filepath.Join(decisionDirectory, "ADR-001-use-a-local-adr.md"), []byte(decision), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if includeGitHabits {
		if err := os.WriteFile(filepath.Join(root, GitHabitsFilename), []byte("version: 1\nlegacy-profile: autonomous\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readFixtureFiles(t *testing.T, root string, paths ...string) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte, len(paths))
	for _, relative := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		result[relative] = data
	}
	return result
}
