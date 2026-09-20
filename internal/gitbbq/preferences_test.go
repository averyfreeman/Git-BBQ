package gitbbq

import (
	"path/filepath"
	"testing"
)

func TestPreferencesRoundTripUsesExplicitOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("GIT_BBQ_CONFIG", path)
	if err := SavePreferences(Preferences{Profile: "autonomous", Languages: []string{"go", "python"}}); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPreferences()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Profile != "autonomous" || len(loaded.Languages) != 2 {
		t.Fatalf("loaded = %#v", loaded)
	}
}
