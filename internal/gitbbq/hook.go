package gitbbq

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RequiredHookEvents lists the lifecycle events required by a Git BBQ project.
var RequiredHookEvents = []string{"UserPromptSubmit", "PreToolUse", "PostToolUse", "PostCompact", "Stop"}

// HookEvent is the host-provided event envelope accepted by the hook adapter.
type HookEvent struct {
	Event     string `json:"event"`
	Workspace string `json:"workspace,omitempty"`
	CWD       string `json:"cwd,omitempty"`
	Payload   any    `json:"payload,omitempty"`
}

// HookConfig records whether hooks are enabled and which lifecycle events they
// handle.
type HookConfig struct {
	Version int      `json:"version"`
	Events  []string `json:"events"`
	Enabled bool     `json:"enabled"`
}

// HookResult reports whether an event is recognized and whether the host may
// continue the associated operation.
type HookResult struct {
	Event   string `json:"event"`
	Active  bool   `json:"active"`
	Allow   bool   `json:"allow"`
	Message string `json:"message"`
}

// DefaultHookConfig returns the enabled configuration with all required events.
func DefaultHookConfig() HookConfig {
	return HookConfig{Version: 1, Events: append([]string(nil), RequiredHookEvents...), Enabled: true}
}

// ReadHookConfig reads and validates root's lifecycle hook configuration.
func ReadHookConfig(root string) (HookConfig, error) {
	var config HookConfig
	data, err := os.ReadFile(filepath.Join(root, HookConfigPath))
	if err != nil {
		return HookConfig{}, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return HookConfig{}, fmt.Errorf("parse %s: %w", HookConfigPath, err)
	}
	if err := ValidateHookConfig(config); err != nil {
		return HookConfig{}, err
	}
	return config, nil
}

// ValidateHookConfig requires version 1, enabled hooks, and the complete
// required event set.
func ValidateHookConfig(config HookConfig) error {
	if config.Version != 1 {
		return fmt.Errorf("unsupported hook configuration version %d", config.Version)
	}
	if !config.Enabled {
		return fmt.Errorf("Git BBQ hooks must remain enabled")
	}
	if !sameStrings(config.Events, RequiredHookEvents) {
		return fmt.Errorf("hook configuration must contain all five lifecycle events")
	}
	return nil
}

// HandleHook accepts a recognized required event and denies an unknown event.
// It does not execute the underlying host operation.
func HandleHook(event HookEvent) HookResult {
	for _, required := range RequiredHookEvents {
		if event.Event == required {
			return HookResult{Event: event.Event, Active: true, Allow: true, Message: fmt.Sprintf("Git BBQ hook %s accepted.", event.Event)}
		}
	}
	return HookResult{Event: event.Event, Active: false, Allow: false, Message: "unknown Git BBQ lifecycle hook; operation denied"}
}

// HandleHookInWorkspace validates the project manifest and hook configuration
// before handling an event. Invalid or missing workspace state is denied.
func HandleHookInWorkspace(root string, event HookEvent) HookResult {
	if root == "" {
		return HookResult{Event: event.Event, Active: false, Allow: false, Message: "missing Git BBQ workspace; operation denied"}
	}
	if _, err := loadManifest(root); err != nil {
		return HookResult{Event: event.Event, Active: false, Allow: false, Message: fmt.Sprintf("Git BBQ manifest is invalid: %v", err)}
	}
	if _, err := ReadHookConfig(root); err != nil {
		return HookResult{Event: event.Event, Active: false, Allow: false, Message: fmt.Sprintf("Git BBQ hooks are invalid: %v", err)}
	}
	return HandleHook(event)
}

// RenderCodexHookResponse converts a hook result to the Codex hook response
// shape. An allowed result produces an empty response object.
func RenderCodexHookResponse(result HookResult) map[string]any {
	if result.Allow {
		return map[string]any{}
	}
	switch result.Event {
	case "PreToolUse":
		return map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":            "PreToolUse",
				"permissionDecision":       "deny",
				"permissionDecisionReason": result.Message,
			},
			"systemMessage": result.Message,
		}
	case "PostToolUse", "Stop":
		return map[string]any{
			"continue":      false,
			"stopReason":    result.Message,
			"systemMessage": result.Message,
		}
	default:
		return map[string]any{
			"continue":      false,
			"stopReason":    result.Message,
			"systemMessage": result.Message,
		}
	}
}
