package gitbbq

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMarkRemoteConfiguredPersistsVerifiedStatus(t *testing.T) {
	root := t.TempDir()
	config := DefaultGitHabits()
	config.Remote.URL = "https://github.com/example/project.git"
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, GitHabitsFilename), data, 0o644); err != nil {
		t.Fatal(err)
	}

	updated, err := MarkRemoteConfigured(root)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Remote.Status != "configured" {
		t.Fatalf("updated remote status = %q", updated.Remote.Status)
	}

	persisted, err := ReadGitHabits(root)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Remote.Status != "configured" || persisted.Remote.URL != config.Remote.URL {
		t.Fatalf("persisted remote = %#v", persisted.Remote)
	}
}

func TestMarkRemoteConfiguredPersistsDerivedURL(t *testing.T) {
	root := t.TempDir()
	config := DefaultGitHabits()
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, GitHabitsFilename), data, 0o644); err != nil {
		t.Fatal(err)
	}

	persisted, err := MarkRemoteConfiguredWithURL(root, "https://github.com/example/project.git")
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Remote.Status != "configured" || persisted.Remote.URL != "https://github.com/example/project.git" {
		t.Fatalf("persisted remote = %#v", persisted.Remote)
	}
}
