package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/averyfreeman/git-bbq/internal/gitbbq"
	"gopkg.in/yaml.v3"
)

func TestRunGithabitsExecutePersistsConfiguredRemote(t *testing.T) {
	root := t.TempDir()
	config, err := gitbbq.GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}
	config.Remote.URL = "https://github.com/example/project.git"
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, gitbbq.GitHabitsFilename), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", root, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}

	if err := runGithabitsExecute([]string{"--approve", "--action", "remote", root, "--json"}); err != nil {
		t.Fatal(err)
	}

	persisted, err := gitbbq.ReadGitHabits(root)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Remote.Status != "configured" {
		t.Fatalf("remote status = %q", persisted.Remote.Status)
	}
}

func TestRunGithabitsPlanRejectsOlderObservedTag(t *testing.T) {
	root := t.TempDir()
	config, err := gitbbq.GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, gitbbq.GitHabitsFilename), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runGithabitsPlan([]string{
		"--action", "tag",
		"--tag", "v0.1.0",
		"--existing-tag", "v0.1.1",
		root,
		"--json",
	}); err == nil || !strings.Contains(err.Error(), "not newer") {
		t.Fatalf("older observed tag error = %v", err)
	}
}

func TestRunGithabitsPlanBuildsRemoteProvisionPlan(t *testing.T) {
	root := t.TempDir()
	config, err := gitbbq.GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}
	config.Remote.Owner = "averyfreeman"
	config.Remote.Name = "example-project"
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, gitbbq.GitHabitsFilename), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runGithabitsPlan([]string{"--action", "remote", "--provision-remote", root, "--json"}); err != nil {
		t.Fatal(err)
	}
}

func TestRunMigrateReportsReadOnlyAssessment(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".adr-scaffold.yaml"), []byte("version: 1\nlanguage: go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runMigrate([]string{"--json", root}); err != nil {
		t.Fatal(err)
	}
}

func TestRunMigrateAppliesOnlyWithApproval(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".adr-scaffold.yaml"), []byte("version: 1\nlanguage: go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".ai-architect"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ai-architect", "project-context.md"), []byte("---\nlanguage: go\nmotivation:\n  problem: Keep migration explicit.\n---\n\n# Context\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, gitbbq.GitHabitsFilename), []byte("version: 1\nbranch: main\nversioning: semver\ninitial_tag: v0.1.0\ncommit_style: conventional-commits\ninit: false\ncommit: false\ntag: false\npush: false\ncreate_remote: false\nremote:\n  visibility: private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runMigrate([]string{"--approve", "--force", "--json", root}); err != nil {
		t.Fatal(err)
	}
	if err := gitbbq.ValidateProject(root); err != nil {
		t.Fatalf("approved migration produced invalid project: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".gitbbq", "migration", "legacy", gitbbq.GitHabitsFilename)); err != nil {
		t.Fatalf("legacy Git habits archive missing: %v", err)
	}
}

func TestRunUninstallDefaultsToAssessmentAndRequiresApproval(t *testing.T) {
	root := t.TempDir()
	if _, err := gitbbq.ScaffoldProject(root, gitbbq.ScaffoldOptions{ProjectName: "Example", Problem: "Remove only owned files.", Languages: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	if err := runUninstall([]string{"--json", root}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, gitbbq.OwnershipFilename)); err != nil {
		t.Fatalf("assessment changed ownership ledger: %v", err)
	}
	if err := runUninstall([]string{"--approve", "--json", root}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, gitbbq.OwnershipFilename)); !os.IsNotExist(err) {
		t.Fatalf("approved uninstall left ownership ledger or returned unexpected error: %v", err)
	}
}

func TestSecureWebPrinterExampleCreatesDocumentedProject(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Secure-Web-Printer")
	problem := "Build Secure-Web-Printer, a Go HTTPS server with an embedded Let's Encrypt ACME requester, HTMX and Markdown templates, and release binaries for aarch64-darwin, gnu-linux-x86_64, gnu-linux-aarch64, Windows x86_64, and Windows aarch64."
	adrTitle := "Set Secure-Web-Printer release targets"
	adrContext := "Secure-Web-Printer needs one release matrix for its Go HTTPS server across macOS arm64, Linux x86_64 and arm64, and Windows x86_64 and arm64."
	adrDecision := "Use darwin/arm64 (aarch64-darwin), linux/amd64 (gnu-linux-x86_64), linux/arm64 (gnu-linux-aarch64), windows/amd64 (Windows x86_64), and windows/arm64 (Windows aarch64)."
	adrWhy := "The names make the release promise readable while Go's GOOS and GOARCH pairs keep builds reproducible."

	if err := runInit([]string{
		"--name", "Secure-Web-Printer",
		"--problem", problem,
		"--language", "go",
		root,
	}); err != nil {
		t.Fatalf("init example: %v", err)
	}
	if err := runADRNew([]string{
		"--title", adrTitle,
		"--context", adrContext,
		"--decision", adrDecision,
		"--why", adrWhy,
		root,
	}); err != nil {
		t.Fatalf("adr example: %v", err)
	}
	if err := runProject([]string{root}); err != nil {
		t.Fatalf("project example: %v", err)
	}
	if err := runValidate([]string{root}); err != nil {
		t.Fatalf("validate example: %v", err)
	}

	manifest, err := gitbbq.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ProjectName != "Secure-Web-Printer" || manifest.Problem != problem {
		t.Fatalf("manifest identity = %#v", manifest)
	}
	if len(manifest.Languages) != 1 || manifest.Languages[0] != "go" {
		t.Fatalf("manifest languages = %#v", manifest.Languages)
	}

	adrPath := filepath.Join(root, gitbbq.ADRDirectory, "0001-set-secure-web-printer-release-targets.md")
	adr, err := gitbbq.ParseADR(adrPath)
	if err != nil {
		t.Fatal(err)
	}
	if adr.Title != adrTitle || !strings.Contains(adr.Body, adrDecision) || !strings.Contains(adr.Body, adrWhy) {
		t.Fatalf("ADR = %#v", adr)
	}

	for _, relative := range []string{
		".agents/mattpocock/DEPENDENCY.yaml",
		".agents/skills/githabits/SKILL.md",
		".agents/skills/go/SKILL.md",
		".gitbbq-manifest.yaml",
		".gitbbq/.gitignore",
		".gitbbq/hooks.json",
		".gitbbq/ownership.json",
		".gitbbq/session.json",
		".githabits.yaml",
		"AGENTS.md",
		"CLAUDE.md",
		"CONTEXT-MAP.md",
		"CONTEXT.md",
		"architecture-contract.yaml",
		"docs/adr/0001-set-secure-web-printer-release-targets.md",
		"docs/adr/index.json",
		"implementation-plan.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("missing documented generated file %s: %v", relative, err)
		}
	}

	contract, err := os.ReadFile(filepath.Join(root, gitbbq.ContractFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contract), "Secure-Web-Printer") || !strings.Contains(string(contract), "gnu-linux-aarch64") {
		t.Fatalf("architecture contract omits example scope: %s", contract)
	}
	plan, err := os.ReadFile(filepath.Join(root, gitbbq.ImplementationPlanFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plan), adrTitle) || !strings.Contains(string(plan), "go") {
		t.Fatalf("implementation projection omits example decision: %s", plan)
	}
	index, err := os.ReadFile(filepath.Join(root, gitbbq.ADRIndexFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), adrTitle) || !strings.Contains(string(index), "\"decision_count\": 1") {
		t.Fatalf("ADR index omits example decision: %s", index)
	}
	ownership, err := os.ReadFile(filepath.Join(root, gitbbq.OwnershipFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ownership), "docs/adr/0001-set-secure-web-printer-release-targets.md") {
		t.Fatalf("ownership ledger omits example ADR: %s", ownership)
	}
	if err := gitbbq.ValidateProject(root); err != nil {
		t.Fatalf("final example validation: %v", err)
	}
}
