package gitbbq

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var conventionalCommitPattern = regexp.MustCompile(`^(feat|fix|docs|refactor|test|chore|build|ci|perf|revert)(\([^)]*\))?!?: .+`)
var semanticVersionTagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

// GitPlanRequest contains the values needed to render one Git action preview.
// It deliberately carries data, not permission; approval remains a host concern.
type GitPlanRequest struct {
	Branch       string
	Message      string
	Tag          string
	Paths        []string
	ExistingTags []string
}

// GitPlan is the read-only result of applying githabits policy to one action.
type GitPlan struct {
	SchemaVersion    string    `json:"schema_version"`
	Action           GitAction `json:"action"`
	Allowed          bool      `json:"allowed"`
	RequiresApproval bool      `json:"requires_approval"`
	Reason           string    `json:"reason"`
	Command          []string  `json:"command"`
}

// PlanGitAction renders one safe Git command without executing it.
func PlanGitAction(config GitHabits, action GitAction, request GitPlanRequest) (GitPlan, error) {
	if err := ValidateGitHabits(config); err != nil {
		return GitPlan{}, err
	}

	plan := GitPlan{
		SchemaVersion:    SchemaVersion,
		Action:           action,
		Allowed:          config.Allows(action),
		RequiresApproval: true,
	}
	if plan.Allowed {
		plan.Reason = "githabits policy allows this action; host approval is still required"
	} else {
		plan.Reason = fmt.Sprintf("githabits policy does not authorize %s", action)
	}

	switch action {
	case GitActionInit:
		plan.Command = []string{"git", "init"}
	case GitActionBranch:
		branch := strings.TrimSpace(request.Branch)
		if branch == "" {
			branch = config.Branch
		}
		if !safeGitRef(branch) {
			return GitPlan{}, fmt.Errorf("branch is not a safe Git ref: %q", branch)
		}
		plan.Command = []string{"git", "switch", "-c", branch}
	case GitActionRemote:
		if config.Remote.Status != "pending" {
			return GitPlan{}, fmt.Errorf("remote planning requires a pending remote")
		}
		remoteURL := strings.TrimSpace(config.Remote.URL)
		if !safeGitRemoteURL(remoteURL) {
			return GitPlan{}, fmt.Errorf("remote URL is empty or unsafe")
		}
		plan.Command = []string{"git", "remote", "add", config.Remote.Alias, remoteURL}
	case GitActionStage:
		if len(request.Paths) == 0 {
			return GitPlan{}, fmt.Errorf("stage planning requires at least one path")
		}
		command := []string{"git", "add", "--"}
		for _, path := range request.Paths {
			if !safeGitPath(path) {
				return GitPlan{}, fmt.Errorf("stage path is not a safe repository-relative path: %q", path)
			}
			command = append(command, path)
		}
		plan.Command = command
	case GitActionTag:
		tag := strings.TrimSpace(request.Tag)
		if tag == "" {
			return GitPlan{}, fmt.Errorf("tag planning requires a tag name")
		}
		if !safeGitRef(tag) {
			return GitPlan{}, fmt.Errorf("tag is not a safe Git ref: %q", tag)
		}
		if config.Versioning == "semver" && !safeSemanticVersionTag(tag) {
			return GitPlan{}, fmt.Errorf("tag does not follow the configured semver convention: %q", tag)
		}
		if err := ValidateTagProgression(request.ExistingTags, tag); err != nil {
			return GitPlan{}, err
		}
		plan.Command = []string{"git", "tag", "-a", tag, "-m", tag}
	case GitActionPush:
		if !plan.Allowed {
			return plan, nil
		}
		branch := strings.TrimSpace(request.Branch)
		if branch == "" {
			branch = config.Branch
		}
		if !safeGitRef(branch) {
			return GitPlan{}, fmt.Errorf("push branch is not a safe Git ref: %q", branch)
		}
		plan.Command = []string{"git", "push", config.Remote.Alias, branch}
	case GitActionCommit:
		message := strings.TrimSpace(request.Message)
		if message == "" {
			return GitPlan{}, fmt.Errorf("commit planning requires a message")
		}
		if config.CommitStyle == "conventional-commits" && !conventionalCommitPattern.MatchString(message) {
			return GitPlan{}, fmt.Errorf("commit message does not follow conventional-commits style")
		}
		plan.Command = []string{"git", "commit", "-m", message}
	default:
		return GitPlan{}, fmt.Errorf("Git action %q is not supported by the planner yet", action)
	}
	if !plan.Allowed {
		plan.Command = nil
	}
	return plan, nil
}

func safeGitPath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "\\") {
		return false
	}
	if len(value) >= 2 && value[1] == ':' {
		return false
	}
	if strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return false
		}
	}
	return !strings.Contains(value, "..\\") && !strings.Contains(value, "../")
}

func safeGitRemoteURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "-") || strings.ContainsAny(value, " \t\x00\r\n") {
		return false
	}
	parsed, err := url.Parse(value)
	return err != nil || parsed.User == nil
}

func safeSemanticVersionTag(value string) bool {
	return semanticVersionTagPattern.MatchString(strings.TrimSpace(value))
}
