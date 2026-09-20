package gitbbq

import (
	"fmt"
	"os"
	"path/filepath"
)

type Preferences struct {
	Version   int      `yaml:"version" json:"version"`
	Profile   string   `yaml:"profile" json:"profile"`
	Languages []string `yaml:"languages,omitempty" json:"languages,omitempty"`
}

func PreferencesPath() (string, error) {
	if override := os.Getenv("GIT_BBQ_CONFIG"); override != "" {
		return filepath.Clean(override), nil
	}
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve Git BBQ user configuration directory: %w", err)
	}
	return filepath.Join(directory, "git-bbq", "config.yaml"), nil
}

func LoadPreferences() (Preferences, error) {
	path, err := PreferencesPath()
	if err != nil {
		return Preferences{}, err
	}
	var preferences Preferences
	if err := readYAML(path, &preferences); err != nil {
		if os.IsNotExist(err) {
			return Preferences{}, nil
		}
		return Preferences{}, err
	}
	if preferences.Version < 1 {
		return Preferences{}, fmt.Errorf("Git BBQ preferences version must be positive")
	}
	if preferences.Profile != "" {
		if _, err := GitHabitsForProfile(preferences.Profile); err != nil {
			return Preferences{}, err
		}
	}
	preferences.Languages = sortedLanguages(preferences.Languages)
	return preferences, nil
}

func SavePreferences(preferences Preferences) error {
	if preferences.Version == 0 {
		preferences.Version = 1
	}
	if preferences.Profile == "" {
		return fmt.Errorf("Git BBQ preference profile is required")
	}
	if _, err := GitHabitsForProfile(preferences.Profile); err != nil {
		return err
	}
	preferences.Languages = sortedLanguages(preferences.Languages)
	data, err := marshalYAML(preferences)
	if err != nil {
		return err
	}
	path, err := PreferencesPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".git-bbq-preferences-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}
