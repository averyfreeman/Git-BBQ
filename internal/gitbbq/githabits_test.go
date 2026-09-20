package gitbbq

import (
	"reflect"
	"testing"
)

func TestPlanGitActionBuildsApprovedCommitCommand(t *testing.T) {
	config, err := GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}

	plan, err := PlanGitAction(config, GitActionCommit, GitPlanRequest{
		Message: "feat: add scaffold",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Allowed {
		t.Fatalf("commit was not allowed: %#v", plan)
	}
	if !plan.RequiresApproval {
		t.Fatal("the plan omitted the host approval requirement")
	}
	if want := []string{"git", "commit", "-m", "feat: add scaffold"}; !reflect.DeepEqual(plan.Command, want) {
		t.Fatalf("command = %#v, want %#v", plan.Command, want)
	}
}

func TestPlanGitActionBuildsStageCommandForReviewedPaths(t *testing.T) {
	config, err := GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}

	plan, err := PlanGitAction(config, GitActionStage, GitPlanRequest{
		Paths: []string{"internal/gitbbq/githabits.go", "README.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Allowed {
		t.Fatalf("stage was not allowed: %#v", plan)
	}
	if want := []string{"git", "add", "--", "internal/gitbbq/githabits.go", "README.md"}; !reflect.DeepEqual(plan.Command, want) {
		t.Fatalf("command = %#v, want %#v", plan.Command, want)
	}
}

func TestPlanGitActionRejectsAbsoluteStagePaths(t *testing.T) {
	config, err := GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"../secrets.env", `C:\secrets.env`} {
		if _, err := PlanGitAction(config, GitActionStage, GitPlanRequest{Paths: []string{path}}); err == nil {
			t.Fatalf("absolute or escaping path %q was accepted", path)
		}
	}
}

func TestPlanGitActionBuildsAnnotatedTagCommand(t *testing.T) {
	config, err := GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}

	plan, err := PlanGitAction(config, GitActionTag, GitPlanRequest{Tag: "v0.1.1"})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Allowed {
		t.Fatalf("tag was not allowed: %#v", plan)
	}
	if want := []string{"git", "tag", "-a", "v0.1.1", "-m", "v0.1.1"}; !reflect.DeepEqual(plan.Command, want) {
		t.Fatalf("command = %#v, want %#v", plan.Command, want)
	}
}

func TestPlanGitActionRejectsNonSemverTag(t *testing.T) {
	config, err := GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := PlanGitAction(config, GitActionTag, GitPlanRequest{Tag: "release"}); err == nil {
		t.Fatal("non-SemVer tag was accepted")
	}
}

func TestPlanGitActionRejectsNonMonotonicTag(t *testing.T) {
	config, err := GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := PlanGitAction(config, GitActionTag, GitPlanRequest{
		Tag:          "v0.1.0",
		ExistingTags: []string{"v0.1.1"},
	}); err == nil {
		t.Fatal("older tag was accepted after a newer existing tag")
	}
}

func TestPlanGitActionBuildsPushCommandForConfiguredRemote(t *testing.T) {
	config, err := GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}
	config.Remote.Status = "configured"
	config.Remote.URL = "https://github.com/example/project.git"

	plan, err := PlanGitAction(config, GitActionPush, GitPlanRequest{Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Allowed {
		t.Fatalf("push was not allowed: %#v", plan)
	}
	if want := []string{"git", "push", "origin", "main"}; !reflect.DeepEqual(plan.Command, want) {
		t.Fatalf("command = %#v, want %#v", plan.Command, want)
	}
}

func TestPlanGitActionRejectsNonConventionalCommitMessage(t *testing.T) {
	config, err := GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := PlanGitAction(config, GitActionCommit, GitPlanRequest{Message: "add scaffold"}); err == nil {
		t.Fatal("non-conventional commit message was accepted")
	}
}

func TestPlanGitActionDoesNotExposeCommandWhenPolicyDeniesAction(t *testing.T) {
	config, err := GitHabitsForProfile("guided")
	if err != nil {
		t.Fatal(err)
	}

	plan, err := PlanGitAction(config, GitActionCommit, GitPlanRequest{Message: "feat: add scaffold"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Allowed || len(plan.Command) != 0 {
		t.Fatalf("denied plan exposed an executable command: %#v", plan)
	}
}

func TestPlanGitActionBuildsRepositoryInitCommand(t *testing.T) {
	config, err := GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}

	plan, err := PlanGitAction(config, GitActionInit, GitPlanRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Allowed {
		t.Fatalf("init was not allowed: %#v", plan)
	}
	if want := []string{"git", "init"}; !reflect.DeepEqual(plan.Command, want) {
		t.Fatalf("command = %#v, want %#v", plan.Command, want)
	}
}

func TestPlanGitActionBuildsBranchCreationCommand(t *testing.T) {
	config, err := GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}

	plan, err := PlanGitAction(config, GitActionBranch, GitPlanRequest{Branch: "feature/git-bbq"})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Allowed {
		t.Fatalf("branch was not allowed: %#v", plan)
	}
	if want := []string{"git", "switch", "-c", "feature/git-bbq"}; !reflect.DeepEqual(plan.Command, want) {
		t.Fatalf("command = %#v, want %#v", plan.Command, want)
	}
}

func TestPlanGitActionBuildsRemoteAddCommandForPendingRemote(t *testing.T) {
	config, err := GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}
	config.Remote.URL = "https://github.com/example/project.git"

	plan, err := PlanGitAction(config, GitActionRemote, GitPlanRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Allowed {
		t.Fatalf("remote was not allowed: %#v", plan)
	}
	if want := []string{"git", "remote", "add", "origin", "https://github.com/example/project.git"}; !reflect.DeepEqual(plan.Command, want) {
		t.Fatalf("command = %#v, want %#v", plan.Command, want)
	}
}

func TestPlanGitActionRejectsCredentialBearingRemoteURL(t *testing.T) {
	config, err := GitHabitsForProfile("autonomous")
	if err != nil {
		t.Fatal(err)
	}
	config.Remote.URL = "https://token:secret@example.com/project.git"

	if _, err := PlanGitAction(config, GitActionRemote, GitPlanRequest{}); err == nil {
		t.Fatal("credential-bearing remote URL was accepted")
	}
}
