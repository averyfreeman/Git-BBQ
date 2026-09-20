package gitbbq

import (
	"fmt"
	"strings"
)

// MarkRemoteConfigured persists the verified remote state after remote setup succeeds.
func MarkRemoteConfigured(root string) (GitHabits, error) {
	return markRemoteConfigured(root, "")
}

// MarkRemoteConfiguredWithURL persists a provider-derived URL after remote setup succeeds.
func MarkRemoteConfiguredWithURL(root, remoteURL string) (GitHabits, error) {
	return markRemoteConfigured(root, strings.TrimSpace(remoteURL))
}

func markRemoteConfigured(root, remoteURL string) (GitHabits, error) {
	config, err := ReadGitHabits(root)
	if err != nil {
		return GitHabits{}, err
	}
	if config.Remote.Status == "configured" {
		return config, nil
	}
	if config.Remote.Status != "pending" {
		return GitHabits{}, fmt.Errorf("remote status %q cannot be marked configured", config.Remote.Status)
	}
	if remoteURL != "" {
		config.Remote.URL = remoteURL
	}
	if !safeGitRemoteURL(config.Remote.URL) {
		return GitHabits{}, fmt.Errorf("configured remote requires a safe URL")
	}

	config.Remote.Status = "configured"
	data, err := marshalYAML(config)
	if err != nil {
		return GitHabits{}, err
	}
	if _, err := writeGenerated(root, GitHabitsFilename, data, true); err != nil {
		return GitHabits{}, fmt.Errorf("persist configured remote: %w", err)
	}
	if err := recordExistingOwnership(root, []string{GitHabitsFilename}); err != nil {
		return GitHabits{}, fmt.Errorf("record configured remote ownership: %w", err)
	}
	return config, nil
}
