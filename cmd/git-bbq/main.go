package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/averyfreeman/git-bbq/internal/gitbbq"
)

type stringList []string

const maxHookInputBytes = 1 << 20

func (values *stringList) String() string { return strings.Join(*values, ",") }
func (values *stringList) Set(value string) error {
	for _, item := range strings.Split(value, ",") {
		if strings.TrimSpace(item) != "" {
			*values = append(*values, strings.TrimSpace(item))
		}
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = runInit(os.Args[2:])
	case "assess":
		err = runAssess(os.Args[2:])
	case "apply":
		err = runApply(os.Args[2:])
	case "migrate":
		err = runMigrate(os.Args[2:])
	case "uninstall":
		err = runUninstall(os.Args[2:])
	case "adr":
		err = runADR(os.Args[2:])
	case "validate":
		err = runValidate(os.Args[2:])
	case "project":
		err = runProject(os.Args[2:])
	case "githabits":
		err = runGithabits(os.Args[2:])
	case "update":
		err = runUpdate(os.Args[2:])
	case "hook":
		err = runHook(os.Args[2:])
	case "doctor":
		err = runDoctor(os.Args[2:])
	case "schema":
		err = runSchema(os.Args[2:])
	case "version":
		fmt.Printf("%s %s\n", gitbbq.ToolName, gitbbq.ToolVersion)
	default:
		usage()
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`git-bbq - Matt-driven architecture scaffolding and Git habits

Usage:
  git-bbq init [path] --problem TEXT --language go[,python] [--here] [--bootstrap] [--profile guided]
  git-bbq assess [path] [--json]
  git-bbq apply [path] --approve --problem TEXT --language go[,python] [-i]
  git-bbq migrate [path] [--approve] [--force] [--json]
  git-bbq uninstall [path] [--approve] [--json]
  git-bbq adr new [path] --title TITLE --context TEXT --decision TEXT --why TEXT
  git-bbq adr index [path]
  git-bbq validate [path]
  git-bbq project [path]
  git-bbq githabits [path]
  git-bbq githabits plan [path] --action commit --message "feat: ..."
  git-bbq githabits execute [path] --approve --action commit --message "feat: ..."
  git-bbq update [path]
  git-bbq hook <event>
  git-bbq doctor [path]
  git-bbq schema [path]

Use --interactive for prompts and --json for stable machine output.`)
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	problem := fs.String("problem", "", "initial project problem statement")
	projectName := fs.String("name", "", "project name")
	var languages stringList
	fs.Var(&languages, "language", "explicit language profile; repeat or comma-separate")
	here := fs.Bool("here", false, "allow initialization in a non-empty current directory")
	interactive := false
	fs.BoolVar(&interactive, "interactive", false, "ask operational setup questions")
	fs.BoolVar(&interactive, "i", false, "ask operational setup questions")
	bootstrap := fs.Bool("bootstrap", false, "write only operational state before the semantic interview")
	force := fs.Bool("force", false, "replace generated files")
	format := fs.String("format", "text", "output format (text|json)")
	jsonOutput := fs.Bool("json", false, "emit JSON")
	profile := fs.String("profile", "", "Git profile (manual|guided|autonomous)")
	remember := fs.Bool("remember", false, "save profile and languages as global defaults")
	var allowActions stringList
	var denyActions stringList
	fs.Var(&allowActions, "allow-action", "enable a Git action; repeat or comma-separate")
	fs.Var(&denyActions, "deny-action", "disable a Git action; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	preferences, err := gitbbq.LoadPreferences()
	if err != nil {
		return err
	}
	if *profile == "" {
		*profile = projectProfile(root, preferences.Profile)
	}
	if len(languages) == 0 {
		languages = projectLanguages(root, preferences.Languages)
	}
	if interactive {
		var err error
		var save bool
		*problem, languages, *profile, save, err = promptSetup(*problem, languages, *profile)
		if err != nil {
			return err
		}
		*remember = *remember || save
	}
	if *profile == "" {
		*profile = projectProfile(root, preferences.Profile)
	}
	if strings.TrimSpace(*problem) == "" {
		if manifest, manifestErr := gitbbq.ReadManifest(root); manifestErr == nil {
			*problem = manifest.Problem
		}
	}
	if strings.TrimSpace(*problem) == "" {
		return errors.New("--problem is required; use --interactive to answer it")
	}
	if len(languages) == 0 {
		return errors.New("at least one --language is required; language detection is intentionally not implicit")
	}
	overrides, err := parseActionOverrides(allowActions, denyActions)
	if err != nil {
		return err
	}
	result, err := gitbbq.ScaffoldProject(root, gitbbq.ScaffoldOptions{
		ProjectName:     *projectName,
		Problem:         *problem,
		Languages:       languages,
		Profile:         *profile,
		ActionOverrides: overrides,
		Force:           *force,
		Bootstrap:       *bootstrap,
		AllowHere:       *here,
	})
	if err != nil {
		return err
	}
	if *remember {
		if err := gitbbq.SavePreferences(gitbbq.Preferences{Profile: *profile, Languages: languages}); err != nil {
			return err
		}
	}
	return output(result, selectedFormat(*format, *jsonOutput))
}

func runAssess(args []string) error {
	fs := flag.NewFlagSet("assess", flag.ContinueOnError)
	format := fs.String("format", "text", "output format (text|json)")
	jsonOutput := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	assessment, err := gitbbq.Assess(root)
	if err != nil {
		return err
	}
	return output(assessment, selectedFormat(*format, *jsonOutput))
}

func runApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	problem := fs.String("problem", "", "initial project problem statement")
	projectName := fs.String("name", "", "project name")
	var languages stringList
	fs.Var(&languages, "language", "explicit language profile; repeat or comma-separate")
	approve := fs.Bool("approve", false, "approve the reviewed assessment")
	force := fs.Bool("force", false, "replace generated files")
	format := fs.String("format", "text", "output format (text|json)")
	jsonOutput := fs.Bool("json", false, "emit JSON")
	profile := fs.String("profile", "", "Git profile (manual|guided|autonomous)")
	remember := fs.Bool("remember", false, "save profile and languages as global defaults")
	var allowActions stringList
	var denyActions stringList
	fs.Var(&allowActions, "allow-action", "enable a Git action; repeat or comma-separate")
	fs.Var(&denyActions, "deny-action", "disable a Git action; repeat or comma-separate")
	interactive := false
	fs.BoolVar(&interactive, "interactive", false, "ask setup questions")
	fs.BoolVar(&interactive, "i", false, "ask setup questions")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*approve {
		return errors.New("apply requires --approve after reviewing assess output")
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	preferences, err := gitbbq.LoadPreferences()
	if err != nil {
		return err
	}
	if len(languages) == 0 {
		languages = projectLanguages(root, preferences.Languages)
	}
	if interactive {
		var save bool
		*problem, languages, *profile, save, err = promptSetup(*problem, languages, *profile)
		if err != nil {
			return err
		}
		*remember = *remember || save
	}
	if *profile == "" {
		*profile = projectProfile(root, preferences.Profile)
	}
	if strings.TrimSpace(*problem) == "" {
		if manifest, manifestErr := gitbbq.ReadManifest(root); manifestErr == nil {
			*problem = manifest.Problem
		}
	}
	if strings.TrimSpace(*problem) == "" || len(languages) == 0 {
		return errors.New("apply requires --problem and at least one --language")
	}
	overrides, err := parseActionOverrides(allowActions, denyActions)
	if err != nil {
		return err
	}
	if _, err := gitbbq.Assess(root); err != nil {
		return err
	}
	result, err := gitbbq.ScaffoldProject(root, gitbbq.ScaffoldOptions{ProjectName: *projectName, Problem: *problem, Languages: languages, Profile: *profile, ActionOverrides: overrides, Force: *force, AllowHere: true})
	if err != nil {
		return err
	}
	if *remember {
		if err := gitbbq.SavePreferences(gitbbq.Preferences{Profile: *profile, Languages: languages}); err != nil {
			return err
		}
	}
	return output(result, selectedFormat(*format, *jsonOutput))
}

func runMigrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	format := fs.String("format", "text", "output format (text|json)")
	jsonOutput := fs.Bool("json", false, "emit JSON")
	approve := fs.Bool("approve", false, "approve the reviewed migration assessment")
	force := fs.Bool("force", false, "archive and replace an incompatible legacy Git habits file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *force && !*approve {
		return errors.New("migrate --force requires --approve")
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	if *approve {
		result, err := gitbbq.Migrate(root, *force)
		if err != nil {
			return err
		}
		return output(result, selectedFormat(*format, *jsonOutput))
	}
	assessment, err := gitbbq.AssessMigration(root)
	if err != nil {
		return err
	}
	return output(assessment, selectedFormat(*format, *jsonOutput))
}

func runUninstall(args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	format := fs.String("format", "text", "output format (text|json)")
	jsonOutput := fs.Bool("json", false, "emit JSON")
	approve := fs.Bool("approve", false, "approve the reviewed uninstall assessment")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	if !*approve {
		assessment, err := gitbbq.AssessUninstall(root)
		if err != nil {
			return err
		}
		return output(assessment, selectedFormat(*format, *jsonOutput))
	}
	result, err := gitbbq.Uninstall(root)
	if err != nil {
		return err
	}
	return output(result, selectedFormat(*format, *jsonOutput))
}

func runADR(args []string) error {
	if len(args) == 0 {
		return errors.New("adr requires new or index")
	}
	switch args[0] {
	case "new":
		return runADRNew(args[1:])
	case "index":
		root := "."
		if len(args) > 1 {
			root = args[1]
		}
		index, err := gitbbq.IndexADRs(root)
		if err != nil {
			return err
		}
		return output(index, "json")
	default:
		return fmt.Errorf("unknown adr command %q", args[0])
	}
}

func runADRNew(args []string) error {
	fs := flag.NewFlagSet("adr new", flag.ContinueOnError)
	title := fs.String("title", "", "short decision title")
	context := fs.String("context", "", "decision context")
	decision := fs.String("decision", "", "chosen decision")
	why := fs.String("why", "", "reason for the choice")
	status := fs.String("status", "proposed", "proposed|accepted|deprecated")
	interactive := false
	fs.BoolVar(&interactive, "interactive", false, "ask ADR questions")
	fs.BoolVar(&interactive, "i", false, "ask ADR questions")
	format := fs.String("format", "text", "output format (text|json)")
	jsonOutput := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if interactive {
		values := []*string{title, context, decision, why}
		labels := []string{"Title", "Context", "Decision", "Why"}
		for i, value := range values {
			if strings.TrimSpace(*value) == "" {
				answer, err := prompt(labels[i] + ": ")
				if err != nil {
					return err
				}
				*value = answer
			}
		}
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	record, err := gitbbq.CreateADR(root, gitbbq.ADRInput{Title: *title, Context: *context, Decision: *decision, Why: *why, Status: *status})
	if err != nil {
		return err
	}
	return output(record, selectedFormat(*format, *jsonOutput))
}

func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	format := fs.String("format", "text", "output format (text|json)")
	jsonOutput := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	if err := gitbbq.ValidateProject(root); err != nil {
		return err
	}
	return output(map[string]any{"valid": true, "root": root}, selectedFormat(*format, *jsonOutput))
}

func runProject(args []string) error {
	fs := flag.NewFlagSet("project", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", true, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	projection, err := gitbbq.Project(root)
	if err != nil {
		return err
	}
	return output(projection, selectedFormat("text", *jsonOutput))
}

func runGithabits(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "plan":
			return runGithabitsPlan(args[1:])
		case "execute":
			return runGithabitsExecute(args[1:])
		}
	}
	fs := flag.NewFlagSet("githabits", flag.ContinueOnError)
	format := fs.String("format", "text", "output format (text|json)")
	jsonOutput := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	config, err := gitbbq.ReadGitHabits(root)
	if err != nil {
		return err
	}
	return output(config, selectedFormat(*format, *jsonOutput))
}

func runGithabitsPlan(args []string) error {
	fs := flag.NewFlagSet("githabits plan", flag.ContinueOnError)
	action := fs.String("action", "", "Git action to preview")
	provisionRemote := fs.Bool("provision-remote", false, "plan approval-gated GitHub remote creation")
	branch := fs.String("branch", "", "branch override")
	message := fs.String("message", "", "commit message")
	tag := fs.String("tag", "", "tag name")
	format := fs.String("format", "text", "output format (text|json)")
	jsonOutput := fs.Bool("json", false, "emit JSON")
	var paths stringList
	fs.Var(&paths, "path", "repository-relative path to stage; repeat or comma-separate")
	var existingTags stringList
	fs.Var(&existingTags, "existing-tag", "observed SemVer tag; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*action) == "" {
		return errors.New("githabits plan requires --action")
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	config, err := gitbbq.ReadGitHabits(root)
	if err != nil {
		return err
	}
	requestedAction := gitbbq.GitAction(strings.ToLower(strings.TrimSpace(*action)))
	var plan gitbbq.GitPlan
	if *provisionRemote {
		if requestedAction != gitbbq.GitActionRemote {
			return errors.New("--provision-remote requires --action remote")
		}
		plan, err = gitbbq.PlanRemoteProvision(config)
	} else {
		plan, err = gitbbq.PlanGitAction(config, requestedAction, gitbbq.GitPlanRequest{
			Branch:       *branch,
			Message:      *message,
			Tag:          *tag,
			Paths:        paths,
			ExistingTags: existingTags,
		})
	}
	if err != nil {
		return err
	}
	return output(plan, selectedFormat(*format, *jsonOutput))
}

func runGithabitsExecute(args []string) error {
	fs := flag.NewFlagSet("githabits execute", flag.ContinueOnError)
	action := fs.String("action", "", "Git action to execute")
	provisionRemote := fs.Bool("provision-remote", false, "execute approval-gated GitHub remote creation")
	branch := fs.String("branch", "", "branch override")
	message := fs.String("message", "", "commit message")
	tag := fs.String("tag", "", "tag name")
	approve := fs.Bool("approve", false, "approve the planned Git mutation")
	format := fs.String("format", "text", "output format (text|json)")
	jsonOutput := fs.Bool("json", false, "emit JSON")
	var paths stringList
	fs.Var(&paths, "path", "repository-relative path to stage; repeat or comma-separate")
	var existingTags stringList
	fs.Var(&existingTags, "existing-tag", "observed SemVer tag; repeat or comma-separate")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*approve {
		return errors.New("githabits execute requires --approve")
	}
	if strings.TrimSpace(*action) == "" {
		return errors.New("githabits execute requires --action")
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	config, err := gitbbq.ReadGitHabits(root)
	if err != nil {
		return err
	}
	requestedAction := gitbbq.GitAction(strings.ToLower(strings.TrimSpace(*action)))
	var plan gitbbq.GitPlan
	if *provisionRemote {
		if requestedAction != gitbbq.GitActionRemote {
			return errors.New("--provision-remote requires --action remote")
		}
		plan, err = gitbbq.PlanRemoteProvision(config)
	} else {
		plan, err = gitbbq.PlanGitAction(config, requestedAction, gitbbq.GitPlanRequest{
			Branch:       *branch,
			Message:      *message,
			Tag:          *tag,
			Paths:        paths,
			ExistingTags: existingTags,
		})
	}
	if err != nil {
		return err
	}
	var runner gitbbq.GitRunner = gitbbq.OSGitRunner{}
	if len(plan.Command) > 0 && plan.Command[0] == "gh" {
		runner = gitbbq.OSGHRunner{}
	}
	execution, err := gitbbq.ExecuteGitPlan(config, root, plan, *approve, runner)
	if err != nil {
		return err
	}
	if plan.Action == gitbbq.GitActionRemote {
		var stateErr error
		if *provisionRemote {
			remoteURL := fmt.Sprintf("https://github.com/%s/%s.git", strings.TrimSpace(config.Remote.Owner), strings.TrimSpace(config.Remote.Name))
			_, stateErr = gitbbq.MarkRemoteConfiguredWithURL(root, remoteURL)
		} else {
			_, stateErr = gitbbq.MarkRemoteConfigured(root)
		}
		if stateErr != nil {
			return stateErr
		}
	}
	return output(execution, selectedFormat(*format, *jsonOutput))
}

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	repository := fs.String("repository", "", "new Matt skill repository")
	commit := fs.String("commit", "", "new pinned Matt commit")
	approve := fs.Bool("approve", false, "approve updating the dependency pin")
	jsonOutput := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	manifest, err := gitbbq.ReadManifest(root)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*repository) == "" && strings.TrimSpace(*commit) == "" {
		return output(manifest.Matt, selectedFormat("text", *jsonOutput))
	}
	if !*approve {
		return errors.New("update requires --approve when changing the Matt dependency pin")
	}
	if strings.TrimSpace(*repository) == "" {
		*repository = manifest.Matt.Repository
	}
	if strings.TrimSpace(*commit) == "" {
		*commit = manifest.Matt.Commit
	}
	updated, err := gitbbq.UpdateMattDependency(root, gitbbq.MattDependency{Repository: *repository, Commit: *commit, Path: manifest.Matt.Path})
	if err != nil {
		return err
	}
	return output(updated.Matt, selectedFormat("text", *jsonOutput))
}

func runHook(args []string) error {
	if len(args) == 0 {
		return errors.New("hook requires one lifecycle event")
	}
	eventName := args[0]
	fs := flag.NewFlagSet("hook", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "project workspace; defaults to the current directory")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, maxHookInputBytes+1))
	if err != nil {
		return output(gitbbq.RenderCodexHookResponse(gitbbq.HookResult{Event: eventName, Allow: false, Message: fmt.Sprintf("read hook event: %v", err)}), "json")
	}
	if len(raw) > maxHookInputBytes {
		return output(gitbbq.RenderCodexHookResponse(gitbbq.HookResult{Event: eventName, Allow: false, Message: "hook input exceeds the bounded input size"}), "json")
	}
	var event gitbbq.HookEvent
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &event); err != nil {
			return output(gitbbq.RenderCodexHookResponse(gitbbq.HookResult{Event: eventName, Allow: false, Message: fmt.Sprintf("parse hook event: %v", err)}), "json")
		}
	}
	event.Event = eventName
	if event.Workspace == "" {
		event.Workspace = *workspace
	}
	if event.Workspace == "" {
		event.Workspace = event.CWD
	}
	if event.Workspace == "" {
		event.Workspace, _ = os.Getwd()
	}
	result := gitbbq.HandleHookInWorkspace(event.Workspace, event)
	return output(gitbbq.RenderCodexHookResponse(result), "json")
}

func runDoctor(args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}
	if err := gitbbq.ValidateProject(root); err != nil {
		return err
	}
	fmt.Printf("Git BBQ is healthy in %s.\n", root)
	return nil
}

func promptSetup(problem string, languages stringList, profile string) (string, stringList, string, bool, error) {
	if strings.TrimSpace(problem) == "" {
		answer, err := prompt("What problem are we solving? ")
		if err != nil {
			return "", nil, "", false, err
		}
		problem = answer
	}
	if len(languages) == 0 {
		answer, err := prompt("Which languages should be scaffolded (comma-separated)? ")
		if err != nil {
			return "", nil, "", false, err
		}
		_ = languages.Set(answer)
	}
	profilePrompt := profile
	if strings.TrimSpace(profilePrompt) == "" {
		profilePrompt = "guided"
	}
	answer, err := prompt(fmt.Sprintf("Git profile (manual/guided/autonomous) [%s]: ", profilePrompt))
	if err != nil {
		return "", nil, "", false, err
	}
	if strings.TrimSpace(answer) != "" {
		profile = answer
	} else if profile == "" {
		profile = profilePrompt
	}
	rememberAnswer, err := prompt("Remember this profile and language selection globally? [y/N]: ")
	if err != nil {
		return "", nil, "", false, err
	}
	remember := strings.EqualFold(strings.TrimSpace(rememberAnswer), "y") || strings.EqualFold(strings.TrimSpace(rememberAnswer), "yes")
	return problem, languages, profile, remember, nil
}

func prompt(label string) (string, error) {
	fmt.Print(label)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(answer), nil
}

func output(value any, format string) error {
	if format != "text" && format != "json" {
		return fmt.Errorf("unsupported output format %q", format)
	}
	if format == "json" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func selectedFormat(format string, jsonOutput bool) string {
	if jsonOutput {
		return "json"
	}
	return format
}

func parseActionOverrides(allowActions, denyActions stringList) (map[gitbbq.GitAction]bool, error) {
	overrides := make(map[gitbbq.GitAction]bool)
	for _, value := range allowActions {
		action := gitbbq.GitAction(strings.ToLower(strings.TrimSpace(value)))
		if action == "" {
			continue
		}
		if _, exists := overrides[action]; exists {
			return nil, fmt.Errorf("Git action %q was specified more than once", action)
		}
		overrides[action] = true
	}
	for _, value := range denyActions {
		action := gitbbq.GitAction(strings.ToLower(strings.TrimSpace(value)))
		if action == "" {
			continue
		}
		if _, exists := overrides[action]; exists {
			return nil, fmt.Errorf("Git action %q was both allowed and denied", action)
		}
		overrides[action] = false
	}
	return overrides, nil
}

func projectProfile(root, fallback string) string {
	if config, err := gitbbq.ReadGitHabits(root); err == nil && config.Profile != "" {
		return config.Profile
	}
	if fallback != "" {
		return fallback
	}
	return "guided"
}

func projectLanguages(root string, fallback []string) stringList {
	if manifest, err := gitbbq.ReadManifest(root); err == nil && len(manifest.Languages) > 0 {
		return append(stringList(nil), manifest.Languages...)
	}
	return append(stringList(nil), fallback...)
}

func runSchema(args []string) error {
	fs := flag.NewFlagSet("schema", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	paths, err := gitbbq.WriteSchemas(root)
	if err != nil {
		return err
	}
	return output(map[string]any{"written": paths}, "json")
}
