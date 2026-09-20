package gitbbq

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHookRequiresAllFiveLifecycleEvents(t *testing.T) {
	for _, event := range RequiredHookEvents {
		result := HandleHook(HookEvent{Event: event})
		if !result.Active || !result.Allow {
			t.Fatalf("event %s result = %#v", event, result)
		}
	}
	result := HandleHook(HookEvent{Event: "Unknown"})
	if result.Allow || result.Active {
		t.Fatalf("unknown event was allowed: %#v", result)
	}
}

func TestWorkspaceHookFailsClosedWithoutScaffold(t *testing.T) {
	result := HandleHookInWorkspace(t.TempDir(), HookEvent{Event: "Stop"})
	if result.Allow || result.Active {
		t.Fatalf("unscaffolded workspace was allowed: %#v", result)
	}
	response := RenderCodexHookResponse(HandleHookInWorkspace(t.TempDir(), HookEvent{Event: "PreToolUse"}))
	if response["hookSpecificOutput"] == nil {
		t.Fatalf("PreToolUse failure did not render Codex denial: %#v", response)
	}
}

func TestWorkspaceHookFailsClosedForEveryEventResponse(t *testing.T) {
	for _, event := range RequiredHookEvents {
		response := RenderCodexHookResponse(HandleHookInWorkspace(t.TempDir(), HookEvent{Event: event}))
		if event == "PreToolUse" {
			output, ok := response["hookSpecificOutput"].(map[string]any)
			if !ok || output["permissionDecision"] != "deny" {
				t.Fatalf("event %s response = %#v", event, response)
			}
			continue
		}
		if response["continue"] != false || response["stopReason"] == "" {
			t.Fatalf("event %s did not fail closed: %#v", event, response)
		}
	}
}

func TestGeneratedHookConfigContainsExactlyRequiredEvents(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(HookConfigPath)), 0o755); err != nil {
		t.Fatal(err)
	}
	config := DefaultHookConfig()
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, HookConfigPath), data, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadHookConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Events) != len(RequiredHookEvents) {
		t.Fatalf("events = %#v", loaded.Events)
	}
}
