package gitbbq

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteGeneratedRejectsSymlinkedParent(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := writeGenerated(root, "linked/generated.txt", []byte("must stay inside\n"), true); err == nil {
		t.Fatal("generated write followed a symlinked parent")
	}
	if _, err := os.Stat(filepath.Join(outside, "generated.txt")); !os.IsNotExist(err) {
		t.Fatalf("outside generated file exists or returned unexpected error: %v", err)
	}
}
