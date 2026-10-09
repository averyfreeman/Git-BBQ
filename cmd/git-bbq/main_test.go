package main

import (
	"bufio"
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

func TestHelpTopicsCoverEveryPublicCommand(t *testing.T) {
	for _, topic := range helpTopicOrder {
		text, err := helpText(topic)
		if err != nil {
			t.Fatalf("help topic %q: %v", topic, err)
		}
		if !strings.Contains(text, "git-bbq") {
			t.Fatalf("help topic %q omitted command name: %q", topic, text)
		}
	}
	text, err := helpText("")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "git-bbq help [command [subcommand]]") {
		t.Fatalf("top-level help omitted help syntax: %q", text)
	}
}

func TestHelpHooksExplainsCodexTrust(t *testing.T) {
	text, err := helpText("hooks")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"UserPromptSubmit", "PreToolUse", "JSON", "stdin", "/hooks", "re-review"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("hook help omitted %q: %q", expected, text)
		}
	}
}

func TestHelpAliasesResolveNestedTopics(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{args: []string{"help", "hooks"}, want: "hooks"},
		{args: []string{"help", "hook"}, want: "hooks"},
		{args: []string{"adr", "new", "--help"}, want: "adr new"},
		{args: []string{"githabits", "execute", "--help"}, want: "githabits execute"},
	}
	for _, test := range cases {
		if got := helpTopicForArgs(test.args); got != test.want {
			t.Fatalf("helpTopicForArgs(%v) = %q, want %q", test.args, got, test.want)
		}
	}
	if _, err := helpText("missing"); err == nil || !strings.Contains(err.Error(), "git-bbq help") {
		t.Fatalf("unknown help topic error = %v", err)
	}
}

func TestRunInitNonInteractiveReportsLanguageEvidenceAndDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_BBQ_CONFIG", filepath.Join(t.TempDir(), "missing-config.yaml"))
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := gitbbq.SnapshotFiles(root)
	if err != nil {
		t.Fatal(err)
	}

	err = runInit([]string{"--problem", "Build a CLI.", root})
	if err == nil || !strings.Contains(err.Error(), "go") || !strings.Contains(err.Error(), "main.go") || !strings.Contains(err.Error(), "--language") {
		t.Fatalf("language selection error = %v", err)
	}
	after, err := gitbbq.SnapshotFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("non-interactive detection wrote files: before=%v after=%v", before, after)
	}
}

func TestRunInitUsesSavedUserLanguagesWithoutDetection(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "preferences.yaml")
	t.Setenv("GIT_BBQ_CONFIG", configPath)
	if err := gitbbq.SavePreferences(gitbbq.Preferences{Profile: "guided", Languages: []string{"python"}}); err != nil {
		t.Fatal(err)
	}

	if err := runInit([]string{"--problem", "Build a Python service.", "--json", root}); err != nil {
		t.Fatal(err)
	}
	manifest, err := gitbbq.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(manifest.Languages, ",") != "python" {
		t.Fatalf("manifest languages = %#v", manifest.Languages)
	}
}

func TestRunInitUsesSavedProjectLanguagesBeforeDetection(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_BBQ_CONFIG", filepath.Join(t.TempDir(), "missing-preferences.yaml"))
	if _, err := gitbbq.ScaffoldProject(root, gitbbq.ScaffoldOptions{ProjectName: "Example", Problem: "Keep saved language intent.", Languages: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "service.py"), []byte("def run(): pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runInit([]string{"--problem", "Keep the saved profile.", "--here", "--json", root}); err != nil {
		t.Fatal(err)
	}
	manifest, err := gitbbq.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(manifest.Languages, ",") != "go" {
		t.Fatalf("saved project languages were replaced by detected candidates: %#v", manifest.Languages)
	}
}

func TestRunInitExplicitLanguagesOverrideDetectionAndPreferences(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_BBQ_CONFIG", filepath.Join(t.TempDir(), "preferences.yaml"))
	if err := gitbbq.SavePreferences(gitbbq.Preferences{Profile: "guided", Languages: []string{"python"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.py"), []byte("def main(): pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runInit([]string{"--problem", "Use the explicit profile.", "--language", "rust", "--here", "--json", root}); err != nil {
		t.Fatal(err)
	}
	manifest, err := gitbbq.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(manifest.Languages, ",") != "rust" {
		t.Fatalf("explicit language selection was not honored: %#v", manifest.Languages)
	}
}

func TestRunMigratePreviewsWithoutWritingAndAppliesAfterApproval(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".adr-scaffold.yaml"), []byte("version: 1\nlanguage: go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".ai-architect"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runMigrate([]string{"--json", root}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, gitbbq.ManifestFilename)); !os.IsNotExist(err) {
		t.Fatalf("migration preview wrote the manifest: %v", err)
	}

	if err := runMigrate([]string{"--approve", "--json", root}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, gitbbq.ManifestFilename)); err != nil {
		t.Fatalf("approved migration did not write the manifest: %v", err)
	}
}

func TestRunMigrateRequiresApprovalForGitHabitsArchive(t *testing.T) {
	if err := runMigrate([]string{"--archive-legacy-githabits", t.TempDir()}); err == nil || !strings.Contains(err.Error(), "requires --approve") {
		t.Fatalf("archive approval error = %v", err)
	}
}

func TestRunInitInteractiveCancellationWritesNothing(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_BBQ_CONFIG", filepath.Join(t.TempDir(), "missing-config.yaml"))
	if err := os.WriteFile(filepath.Join(root, "main.py"), []byte("def main(): pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := gitbbq.SnapshotFiles(root)
	if err != nil {
		t.Fatal(err)
	}

	stdin, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	originalStdin := os.Stdin
	os.Stdin = stdin
	t.Cleanup(func() {
		os.Stdin = originalStdin
		stdin.Close()
	})
	if _, err := writer.WriteString("\n\nn\nn\n"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	err = runInit([]string{"--interactive", "--here", "--problem", "Build a worker.", root})
	if err == nil || !strings.Contains(err.Error(), "setup cancelled") {
		t.Fatalf("interactive cancellation error = %v", err)
	}
	after, err := gitbbq.SnapshotFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("cancelled interactive setup wrote files: before=%v after=%v", before, after)
	}
}

func TestInteractiveSetupAllowsEditingDetectedLanguageSet(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("rust,java\n\nn\ny\n"))
	problem, languages, profile, remember, err := promptSetupWithReader(
		"Build a compiler.",
		stringList{"go", "python"},
		"guided",
		true,
		"detected candidates",
		reader,
	)
	if err != nil {
		t.Fatal(err)
	}
	if problem != "Build a compiler." || profile != "guided" || remember {
		t.Fatalf("setup values = problem %q, profile %q, remember %v", problem, profile, remember)
	}
	if strings.Join(languages, ",") != "rust,java" {
		t.Fatalf("selected languages = %#v", languages)
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
