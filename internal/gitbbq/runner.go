package gitbbq

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// OSGitRunner is the production adapter for the GitRunner seam.
// It invokes Git directly and never evaluates a shell command line.
type OSGitRunner struct {
	Timeout time.Duration
}

func (runner OSGitRunner) Run(workspace string, command []string) (GitRunResult, error) {
	if len(command) == 0 || command[0] != "git" {
		return GitRunResult{}, fmt.Errorf("Git runner accepts only git commands")
	}
	return runOSCommand(workspace, command, runner.Timeout)
}

// OSGHRunner is the production adapter for approved GitHub CLI commands.
type OSGHRunner struct {
	Timeout time.Duration
}

func (runner OSGHRunner) Run(workspace string, command []string) (GitRunResult, error) {
	if len(command) == 0 || command[0] != "gh" {
		return GitRunResult{}, fmt.Errorf("gh runner accepts only gh commands")
	}
	return runOSCommand(workspace, command, runner.Timeout)
}

func runOSCommand(workspace string, command []string, configuredTimeout time.Duration) (GitRunResult, error) {
	timeout := configuredTimeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	process := exec.CommandContext(ctx, command[0], command[1:]...)
	process.Dir = workspace
	if err := process.Run(); err == nil {
		return GitRunResult{ExitCode: 0}, nil
	} else {
		if ctx.Err() != nil {
			return GitRunResult{}, fmt.Errorf("Git command timed out: %w", ctx.Err())
		}
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return GitRunResult{ExitCode: exitError.ExitCode()}, nil
		}
		return GitRunResult{}, err
	}
}
