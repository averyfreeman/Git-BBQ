package gitbbq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateADRUsesMattSequentialLayout(t *testing.T) {
	root := t.TempDir()
	adrDir := filepath.Join(root, ADRDirectory)
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "0001-existing.md"), []byte("# Existing\n\nWe kept the existing boundary because it is reversible.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	record, err := CreateADR(root, ADRInput{
		Title:    "Choose a durable seam",
		Context:  "The workflow needs one stable integration point.",
		Decision: "Use a narrow command interface.",
		Why:      "It keeps callers small while hiding lifecycle complexity.",
		Status:   "proposed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Path != filepath.Join(root, ADRDirectory, "0002-choose-a-durable-seam.md") {
		t.Fatalf("path = %q", record.Path)
	}
	if strings.Contains(record.Body, "ADR-002") || strings.Contains(record.Body, "schema_version") {
		t.Fatalf("Matt ADR unexpectedly contains native metadata: %s", record.Body)
	}
	if !strings.Contains(record.Body, "We decided: Use a narrow command interface. The reason is: It keeps callers small while hiding lifecycle complexity.") {
		t.Fatalf("ADR rationale is not a natural Matt paragraph: %s", record.Body)
	}
	parsed, err := ParseADR(record.Path)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Number != 2 || parsed.Status != "proposed" || parsed.Title != record.Title {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestAcceptedADRIsNotMutableThroughCreate(t *testing.T) {
	root := t.TempDir()
	first, err := CreateADR(root, ADRInput{Title: "Keep an accepted record", Context: "Context.", Decision: "Keep it.", Why: "It is durable.", Status: "accepted"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateADR(root, ADRInput{Title: first.Title, Context: "Changed.", Decision: "Change it.", Why: "Changed.", Status: "accepted"}); err == nil {
		t.Fatal("duplicate accepted decision was created")
	}
}

func TestADRRejectsInvalidStatusAndEmptySlug(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateADR(root, ADRInput{Title: "???", Context: "Context.", Decision: "Decision.", Why: "Reason.", Status: "accepted"}); err == nil {
		t.Fatal("ADR with an empty slug was accepted")
	}
	if _, err := CreateADR(root, ADRInput{Title: "A decision", Context: "Context.", Decision: "Decision.", Why: "Reason.", Status: "superseded by ADR-1"}); err == nil {
		t.Fatal("malformed supersession status was accepted")
	}
}
