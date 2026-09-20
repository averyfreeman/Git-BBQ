package gitbbq

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var RequiredHookEvents = []string{"UserPromptSubmit", "PreToolUse", "PostToolUse", "PostCompact", "Stop"}

type HookEvent struct {
	Event     string `json:"event"`
	Workspace string `json:"workspace,omitempty"`
	CWD       string `json:"cwd,omitempty"`
	Payload   any    `json:"payload,omitempty"`
}

type HookConfig struct {
	Version int      `json:"version"`
	Events  []string `json:"events"`
	Enabled bool     `json:"enabled"`
}

type HookResult struct {
	Event   string `json:"event"`
	Active  bool   `json:"active"`
	Allow   bool   `json:"allow"`
	Message string `json:"message"`
}

func DefaultHookConfig() HookConfig {
	return HookConfig{Version: 1, Events: append([]string(nil), RequiredHookEvents...), Enabled: true}
}

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

func HandleHook(event HookEvent) HookResult {
	for _, required := range RequiredHookEvents {
		if event.Event == required {
			return HookResult{Event: event.Event, Active: true, Allow: true, Message: fmt.Sprintf("Git BBQ hook %s accepted.", event.Event)}
		}
	}
	return HookResult{Event: event.Event, Active: false, Allow: false, Message: "unknown Git BBQ lifecycle hook; operation denied"}
}

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
