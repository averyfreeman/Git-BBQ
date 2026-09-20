package gitbbq

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ScaffoldProject(root string, options ScaffoldOptions) (ScaffoldResult, error) {
	root = filepath.Clean(root)
	if root == "." {
		root, _ = os.Getwd()
	}
	if err := prepareRoot(root, options.AllowHere); err != nil {
		return ScaffoldResult{}, err
	}
	name := strings.TrimSpace(options.ProjectName)
	if name == "" {
		name = filepath.Base(root)
	}
	languages := sortedLanguages(options.Languages)
	manifest := DefaultManifest(name)
	manifest.Problem = strings.TrimSpace(options.Problem)
	manifest.Languages = languages
	if err := ValidateManifest(manifest); err != nil {
		return ScaffoldResult{}, err
	}
	profile := options.Profile
	if strings.TrimSpace(profile) == "" {
		profile = "guided"
	}
	gitHabits, err := GitHabitsForProfile(profile)
	if err != nil {
		return ScaffoldResult{}, err
	}
	for action, allowed := range options.ActionOverrides {
		if err := gitHabits.SetAction(action, allowed); err != nil {
			return ScaffoldResult{}, err
		}
	}
	if err := ValidateGitHabits(gitHabits); err != nil {
		return ScaffoldResult{}, err
	}
	result := ScaffoldResult{Root: root, Created: []string{}, Skipped: []string{}}
	write := func(relative string, data []byte) error {
		created, err := writeGenerated(root, relative, data, options.Force)
		if err != nil {
			return err
		}
		if created {
			result.Created = append(result.Created, relative)
		} else {
			result.Skipped = append(result.Skipped, relative)
		}
		return nil
	}
	manifestData, err := marshalYAML(manifest)
	if err != nil {
		return ScaffoldResult{}, err
	}
	if err := write(ManifestFilename, manifestData); err != nil {
		return ScaffoldResult{}, err
	}
	gitData, err := marshalYAML(gitHabits)
	if err != nil {
		return ScaffoldResult{}, err
	}
	if err := write(GitHabitsFilename, gitData); err != nil {
		return ScaffoldResult{}, err
	}
	hooks, err := json.MarshalIndent(DefaultHookConfig(), "", "  ")
	if err != nil {
		return ScaffoldResult{}, err
	}
	hooks = append(hooks, '\n')
	if err := write(HookConfigPath, hooks); err != nil {
		return ScaffoldResult{}, err
	}
	if err := write("AGENTS.md", []byte(renderAgents())); err != nil {
		return ScaffoldResult{}, err
	}
	if err := write("CLAUDE.md", []byte("# Claude instructions\n\nRead [AGENTS.md](./AGENTS.md) before working in this repository.\n")); err != nil {
		return ScaffoldResult{}, err
	}
	dependencyData, err := marshalYAML(manifest.Matt)
	if err != nil {
		return ScaffoldResult{}, err
	}
	if err := write(MattDependencyMetadataPath, dependencyData); err != nil {
		return ScaffoldResult{}, err
	}
	if err := write(SessionIgnorePath, []byte("session.json\n")); err != nil {
		return ScaffoldResult{}, err
	}
	sessionData, err := json.MarshalIndent(map[string]any{
		"version":                     1,
		"phase":                       map[bool]string{true: "operational-bootstrap", false: "ready"}[options.Bootstrap],
		"semantic_interview_complete": !options.Bootstrap,
	}, "", "  ")
	if err != nil {
		return ScaffoldResult{}, err
	}
	sessionData = append(sessionData, '\n')
	if err := write(SessionPath, sessionData); err != nil {
		return ScaffoldResult{}, err
	}
	if !options.Bootstrap {
		if err := write(ContextFilename, []byte(renderContext(name))); err != nil {
			return ScaffoldResult{}, err
		}
		if err := write(ContextMapFilename, []byte(renderContextMap())); err != nil {
			return ScaffoldResult{}, err
		}
		if err := write(filepath.ToSlash(filepath.Join(".agents", "skills", "githabits", "SKILL.md")), []byte(renderGithabitsSkill())); err != nil {
			return ScaffoldResult{}, err
		}
		for _, language := range languages {
			path := filepath.ToSlash(filepath.Join(".agents", "skills", language, "SKILL.md"))
			if err := write(path, []byte(renderLanguageSkill(language))); err != nil {
				return ScaffoldResult{}, err
			}
		}
	}
	ownershipCreated, err := writeOwnershipLedger(root, result.Created, options.Force)
	if err != nil {
		return ScaffoldResult{}, err
	}
	if ownershipCreated {
		result.Created = append(result.Created, OwnershipFilename)
	} else {
		result.Skipped = append(result.Skipped, OwnershipFilename)
	}
	sort.Strings(result.Created)
	sort.Strings(result.Skipped)
	return result, nil
}

func Assess(root string) (Assessment, error) {
	root = filepath.Clean(root)
	if info, err := os.Stat(root); err != nil {
		return Assessment{}, err
	} else if !info.IsDir() {
		return Assessment{}, fmt.Errorf("assessment root is not a directory: %s", root)
	}
	existing, err := SnapshotFiles(root)
	if err != nil {
		return Assessment{}, err
	}
	conflicts := make([]string, 0)
	for _, relative := range projectPaths(root) {
		for _, item := range existing {
			if item == relative {
				conflicts = append(conflicts, relative)
				break
			}
		}
	}
	return Assessment{Mode: "assessment", Root: root, Existing: existing, Proposed: projectPaths(root), Conflicts: conflicts, ReadOnly: true}, nil
}

func Project(root string) (Projection, error) {
	projection, _, err := projectWithOptions(root, true)
	return projection, err
}

func projectWithOptions(root string, force bool) (Projection, []string, error) {
	if err := ValidateProject(root); err != nil {
		return Projection{}, nil, err
	}
	manifest, err := loadManifest(root)
	if err != nil {
		return Projection{}, nil, err
	}
	index, indexCreated, err := indexADRs(root, force)
	if err != nil {
		return Projection{}, nil, err
	}
	contract := ArchitectureContract{
		SchemaVersion: SchemaVersion,
		Revision:      1,
		Scope:         manifest.ProjectName,
		Problem:       manifest.Problem,
		Languages:     manifest.Languages,
		ADRFiles:      make([]string, 0, len(index.Decisions)),
	}
	for _, record := range index.Decisions {
		contract.ADRFiles = append(contract.ADRFiles, filepath.ToSlash(filepath.Join(ADRDirectory, record.Filename)))
	}
	contractData, err := marshalYAML(contract)
	if err != nil {
		return Projection{}, nil, err
	}
	contractCreated, err := writeGenerated(root, ContractFilename, contractData, force)
	if err != nil {
		return Projection{}, nil, err
	}
	plan := renderImplementationPlan(manifest, index)
	planCreated, err := writeGenerated(root, ImplementationPlanFilename, []byte(plan), force)
	if err != nil {
		return Projection{}, nil, err
	}
	paths := []string{ContractFilename, ImplementationPlanFilename}
	if index.DecisionCount > 0 {
		paths = append(paths, ADRIndexFilename)
	}
	created := make([]string, 0, 3)
	if indexCreated {
		created = append(created, ADRIndexFilename)
	}
	if contractCreated {
		created = append(created, ContractFilename)
	}
	if planCreated {
		created = append(created, ImplementationPlanFilename)
	}
	if err := recordGeneratedOwnership(root, created); err != nil {
		return Projection{}, nil, err
	}
	return Projection{ADRCount: index.DecisionCount, Paths: paths}, created, nil
}

func SnapshotFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		relative := relativePath(root, path)
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == ".gitbbq") {
			return filepath.SkipDir
		}
		if !entry.IsDir() {
			files = append(files, relative)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func prepareRoot(root string, allowHere bool) error {
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return os.MkdirAll(root, 0o755)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("project root is not a directory: %s", root)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	if len(entries) > 0 && !allowHere {
		return fmt.Errorf("existing directory requires explicit --here: %s", root)
	}
	return nil
}

func renderAgents() string {
	return "# Agent instructions\n\nRead [CONTEXT.md](./CONTEXT.md) for project vocabulary, then consult relevant records under [docs/adr](./docs/adr).\n\nFocused operating skills live under `.agents/skills/`. The Matt Pocock skills dependency is pinned under `.agents/mattpocock/`.\n\nUse `git-bbq validate` before persisting architecture projections. Git behavior is governed by `.githabits.yaml`.\n"
}

func renderContext(name string) string {
	return fmt.Sprintf("# %s\n\nThis context records the project vocabulary. Keep definitions precise and free of implementation details.\n\n## Language\n\nAdd project-specific terms here as Matt domain-modeling work resolves them.\n", name)
}

func renderContextMap() string {
	return "# Context map\n\n- System context: [CONTEXT.md](./CONTEXT.md)\n- System decisions: [docs/adr/](./docs/adr/)\n"
}

func renderGithabitsSkill() string {
	return "---\nname: githabits\ndescription: Apply the repository's explicit Git workflow policy.\n---\n\n# Githabits\n\nRead `.githabits.yaml` before any Git mutation. The profile is only a default; the action-level switches are authoritative and may revoke autonomous behavior.\n\n## Action contract\n\n- `init`: initialize Git only when `actions.init` is enabled.\n- `branch`: create or switch branches using the configured branch convention.\n- `stage`: stage only the reviewed repository changes; never stage secrets or unrelated paths.\n- `commit`: use the configured commit style and explain the material change.\n- `tag`: use the configured SemVer tag convention, beginning at `initial_tag`.\n- `remote`: configure the declared provider and alias only after the remote is known.\n- `push`: push only when `actions.push` is enabled, the remote status is `configured`, and validation has passed.\n\nAll actions remain subject to user approval, repository safety checks, and the\nCodex host's permissions. Never expose credentials, rewrite shared history,\nforce-push, delete branches, or weaken validation to make a workflow pass.\n"
}

func renderLanguageSkill(language string) string {
	name := languageDisplayName(language)
	return fmt.Sprintf("---\nname: %s\ndescription: Project-specific %s conventions.\n---\n\n# %s\n\nUse the repository's existing structure and keep changes aligned with approved Matt architecture decisions. Load this skill only when working with %s code.\n", language, language, name, language)
}

func languageDisplayName(language string) string {
	switch language {
	case "go":
		return "Go"
	case "python":
		return "Python"
	case "typescript":
		return "TypeScript"
	case "javascript":
		return "JavaScript"
	case "rust":
		return "Rust"
	case "java":
		return "Java"
	case "csharp":
		return "C#"
	default:
		return language
	}
}

func renderImplementationPlan(manifest Manifest, index ADRIndex) string {
	var builder strings.Builder
	builder.WriteString("# Implementation plan\n\n")
	builder.WriteString("This projection is derived from Matt-native context and ADR documents.\n\n")
	builder.WriteString("## Problem\n\n")
	builder.WriteString(manifest.Problem)
	builder.WriteString("\n\n## Languages\n\n")
	builder.WriteString(strings.Join(manifest.Languages, ", "))
	builder.WriteString("\n\n## Decisions\n\n")
	if len(index.Decisions) == 0 {
		builder.WriteString("No ADRs have been recorded yet.\n")
		return builder.String()
	}
	for _, record := range index.Decisions {
		builder.WriteString("- [")
		builder.WriteString(record.Title)
		builder.WriteString("](")
		builder.WriteString(filepath.ToSlash(filepath.Join(ADRDirectory, record.Filename)))
		builder.WriteString(") — ")
		builder.WriteString(record.Status)
		builder.WriteString("\n")
	}
	return builder.String()
}
