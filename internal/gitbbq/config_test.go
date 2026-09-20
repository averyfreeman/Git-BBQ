package gitbbq

import (
	"strings"
	"testing"
)

func TestGitHabitsExposeGranularActions(t *testing.T) {
	config, err := GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}
	config.Remote.Status = "configured"
	config.Remote.URL = "https://github.com/example/project.git"
	for _, action := range []GitAction{GitActionStage, GitActionCommit, GitActionTag, GitActionPush} {
		if !config.Allows(action) {
			t.Fatalf("action %s was not enabled", action)
		}
	}
	if config.CommitStyle != "conventional-commits" || config.Versioning != "semver" {
		t.Fatalf("defaults = %#v", config)
	}
	if err := ValidateGitHabits(config); err != nil {
		t.Fatal(err)
	}
}

func TestGuidedProfileDoesNotAuthorizeGitMutations(t *testing.T) {
	config, err := GitHabitsForProfile("guided")
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []GitAction{GitActionInit, GitActionStage, GitActionCommit, GitActionTag, GitActionRemote, GitActionPush} {
		if config.Allows(action) {
			t.Fatalf("guided profile authorized %s", action)
		}
	}
}

func TestConfiguredRemoteRequiresURL(t *testing.T) {
	config := DefaultGitHabits()
	config.Remote.Status = "configured"
	if err := ValidateGitHabits(config); err == nil || !strings.Contains(err.Error(), "URL") {
		t.Fatalf("configured remote without URL was accepted: %v", err)
	}
}

func TestPendingRemoteBlocksPushEvenInAutonomousProfile(t *testing.T) {
	config, err := GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}
	if config.Actions.Push == false || config.Allows(GitActionPush) {
		t.Fatalf("pending remote did not block push: %#v", config)
	}
}

func TestManifestRequiresProblemAndExplicitLanguages(t *testing.T) {
	manifest := DefaultManifest("Example")
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "problem") {
		t.Fatal("manifest without problem was accepted")
	}
	manifest.Problem = "Agents need a stable scaffold."
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "language") {
		t.Fatal("manifest without languages was accepted")
	}
	manifest.Languages = []string{"go"}
	if err := ValidateManifest(manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Languages = []string{"java", "csharp"}
	if err := ValidateManifest(manifest); err != nil {
		t.Fatal(err)
	}
}
