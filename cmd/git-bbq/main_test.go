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
