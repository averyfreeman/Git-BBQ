package gitbbq

import (
	"testing"
	"time"
)

func TestOSGHRunnerRejectsNonGhCommand(t *testing.T) {
	_, err := (OSGHRunner{Timeout: time.Second}).Run(t.TempDir(), []string{"git", "status"})
	if err == nil {
		t.Fatal("Git command was accepted by the gh runner")
	}
}
