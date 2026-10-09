package gitbbq

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// ToolName is the executable name used by Git BBQ.
	ToolName = "git-bbq"
	// ToolVersion is the current Git BBQ CLI version.
	ToolVersion = "0.4.1"
	// SchemaVersion is the version shared by Git BBQ project contracts.
	SchemaVersion = "1.0.0"
	// ManifestFilename is the project-relative path of the Git BBQ manifest.
	ManifestFilename = ".gitbbq-manifest.yaml"
	// GitHabitsFilename is the project-relative path of the Git policy.
	GitHabitsFilename = ".githabits.yaml"
	// ContextFilename is the path of the project vocabulary document.
	ContextFilename = "CONTEXT.md"
	// ContextMapFilename is the path of the project context routing document.
	ContextMapFilename = "CONTEXT-MAP.md"
	// ADRDirectory contains the project's canonical architecture decisions.
	ADRDirectory = "docs/adr"
	// ADRIndexFilename is the generated retrieval index for architecture decisions.
	ADRIndexFilename = "docs/adr/index.json"
	// ContractFilename is the generated architecture contract projection.
	ContractFilename = "architecture-contract.yaml"
	// ImplementationPlanFilename is the generated implementation plan projection.
	ImplementationPlanFilename = "implementation-plan.md"
	// HookConfigPath is the path of the generated lifecycle hook configuration.
	HookConfigPath = ".gitbbq/hooks.json"
	// SessionPath is the path of Git BBQ's per-project operational session state.
	SessionPath = ".gitbbq/session.json"
	// SessionIgnorePath is the path of the ignore rule for per-project session state.
	SessionIgnorePath = ".gitbbq/.gitignore"
	// OwnershipFilename records which generated files Git BBQ owns.
	OwnershipFilename = ".gitbbq/ownership.json"
	// MattDependencyMetadataPath records the upstream skills dependency pin.
	MattDependencyMetadataPath = ".agents/mattpocock/DEPENDENCY.yaml"
	// MattDependencyPath is the generated project's dependency checkout path.
	MattDependencyPath = ".agents/mattpocock"
	// MattRepository is the upstream Matt Pocock skills Git repository.
	MattRepository = "https://github.com/mattpocock/skills.git"
	// MattCommit is the upstream skills revision used by new project manifests.
	MattCommit = "b0618bc436ad893b3c5e84e55fba86586d34a404"
)

// SupportedLanguages lists the canonical profile identifiers accepted by Git BBQ.
var SupportedLanguages = []string{"go", "python", "typescript", "javascript", "rust", "java", "csharp"}

// MattDependency identifies the upstream skill repository, revision, and project path.
type MattDependency struct {
	Repository string `yaml:"repository" json:"repository"`
	Commit     string `yaml:"commit" json:"commit"`
	Path       string `yaml:"path" json:"path"`
}

// HookSettings records whether lifecycle hooks are enabled and which events are required.
type HookSettings struct {
	Enabled  bool     `yaml:"enabled" json:"enabled"`
	Required []string `yaml:"required" json:"required"`
}

// Manifest is the canonical Git BBQ project configuration.
type Manifest struct {
	SchemaVersion string         `yaml:"schema_version" json:"schema_version"`
	Version       int            `yaml:"version" json:"version"`
	ProjectName   string         `yaml:"project_name" json:"project_name"`
	Problem       string         `yaml:"problem" json:"problem"`
	Languages     []string       `yaml:"languages" json:"languages"`
	Matt          MattDependency `yaml:"matt" json:"matt"`
	Hooks         HookSettings   `yaml:"hooks" json:"hooks"`
}

// RemoteConfig records the configured Git remote provider and its lifecycle state.
type RemoteConfig struct {
	Provider   string `yaml:"provider" json:"provider"`
	Owner      string `yaml:"owner,omitempty" json:"owner,omitempty"`
	Name       string `yaml:"name,omitempty" json:"name,omitempty"`
	Visibility string `yaml:"visibility" json:"visibility"`
	Alias      string `yaml:"alias" json:"alias"`
	Status     string `yaml:"status" json:"status"`
	URL        string `yaml:"url,omitempty" json:"url,omitempty"`
}

// GitActions contains the per-action permissions used by GitHabits.
type GitActions struct {
	Init   bool `yaml:"init" json:"init"`
	Branch bool `yaml:"branch" json:"branch"`
	Stage  bool `yaml:"stage" json:"stage"`
	Commit bool `yaml:"commit" json:"commit"`
	Tag    bool `yaml:"tag" json:"tag"`
	Remote bool `yaml:"remote" json:"remote"`
	Push   bool `yaml:"push" json:"push"`
}

// GitHabits is the project's explicit policy for planning and executing Git actions.
type GitHabits struct {
	SchemaVersion   string       `yaml:"schema_version" json:"schema_version"`
	Version         int          `yaml:"version" json:"version"`
	Profile         string       `yaml:"profile" json:"profile"`
	Branch          string       `yaml:"branch" json:"branch"`
	Versioning      string       `yaml:"versioning" json:"versioning"`
	InitialTag      string       `yaml:"initial_tag" json:"initial_tag"`
	CommitStyle     string       `yaml:"commit_style" json:"commit_style"`
	SemanticRelease bool         `yaml:"semantic_release" json:"semantic_release"`
	Actions         GitActions   `yaml:"actions" json:"actions"`
	Remote          RemoteConfig `yaml:"remote" json:"remote"`
}

// GitAction names a Git operation supported by the policy planner.
type GitAction string

const (
	// GitActionInit names Git repository initialization.
	GitActionInit GitAction = "init"
	// GitActionBranch names branch creation.
	GitActionBranch GitAction = "branch"
	// GitActionStage names staging reviewed paths.
	GitActionStage GitAction = "stage"
	// GitActionCommit names commit creation.
	GitActionCommit GitAction = "commit"
	// GitActionTag names release tag creation.
	GitActionTag GitAction = "tag"
	// GitActionRemote names remote configuration.
	GitActionRemote GitAction = "remote"
	// GitActionPush names pushing commits to a configured remote.
	GitActionPush GitAction = "push"
)

// DefaultManifest returns a new project manifest with the supplied name and
// the current upstream dependency and required hook defaults.
func DefaultManifest(projectName string) Manifest {
	return Manifest{
		SchemaVersion: SchemaVersion,
		Version:       1,
		ProjectName:   strings.TrimSpace(projectName),
		Languages:     nil,
		Matt: MattDependency{
			Repository: MattRepository,
			Commit:     MattCommit,
			Path:       MattDependencyPath,
		},
		Hooks: HookSettings{Enabled: true, Required: append([]string(nil), RequiredHookEvents...)},
	}
}

// DefaultGitHabits returns the guided policy defaults for a new project.
func DefaultGitHabits() GitHabits {
	return GitHabits{
		SchemaVersion:   SchemaVersion,
		Version:         1,
		Profile:         "guided",
		Branch:          "main",
		Versioning:      "semver",
		InitialTag:      "v0.1.0",
		CommitStyle:     "conventional-commits",
		SemanticRelease: false,
		Remote:          RemoteConfig{Provider: "github", Visibility: "private", Alias: "origin", Status: "pending"},
	}
}

// GitHabitsForProfile returns the policy defaults for manual, guided, or
// autonomous mode. It returns an error for an unsupported profile.
func GitHabitsForProfile(profile string) (GitHabits, error) {
	config := DefaultGitHabits()
	config.Profile = strings.ToLower(strings.TrimSpace(profile))
	switch config.Profile {
	case "manual":
		// Every Git action remains explicitly human-directed.
	case "guided":
		// Guided mode records policy without authorizing mutations by default.
	case "autonomous":
		config.Actions = GitActions{
			Init: true, Branch: true, Stage: true, Commit: true,
			Tag: true, Remote: true, Push: true,
		}
	default:
		return GitHabits{}, fmt.Errorf("unsupported githabits profile %q", profile)
	}
	return config, nil
}

// SetAction changes one action permission and rejects unknown actions.
func (config *GitHabits) SetAction(action GitAction, allowed bool) error {
	switch action {
	case GitActionInit:
		config.Actions.Init = allowed
	case GitActionBranch:
		config.Actions.Branch = allowed
	case GitActionStage:
		config.Actions.Stage = allowed
	case GitActionCommit:
		config.Actions.Commit = allowed
	case GitActionTag:
		config.Actions.Tag = allowed
	case GitActionRemote:
		config.Actions.Remote = allowed
	case GitActionPush:
		config.Actions.Push = allowed
	default:
		return fmt.Errorf("unsupported Git action %q", action)
	}
	return nil
}

// Allows reports whether the policy permits an action. Push also requires a
// configured remote.
func (config GitHabits) Allows(action GitAction) bool {
	switch action {
	case GitActionInit:
		return config.Actions.Init
	case GitActionBranch:
		return config.Actions.Branch
	case GitActionStage:
		return config.Actions.Stage
	case GitActionCommit:
		return config.Actions.Commit
	case GitActionTag:
		return config.Actions.Tag
	case GitActionRemote:
		return config.Actions.Remote
	case GitActionPush:
		return config.Actions.Push && config.Remote.Status == "configured"
	default:
		return false
	}
}

// ValidateManifest checks required project metadata, supported languages,
// dependency fields, and the required lifecycle hook policy.
func ValidateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported manifest schema_version %q", manifest.SchemaVersion)
	}
	if strings.TrimSpace(manifest.SchemaVersion) == "" {
		return fmt.Errorf("manifest schema_version is required")
	}
	if strings.TrimSpace(manifest.ProjectName) == "" {
		return fmt.Errorf("manifest project_name is required")
	}
	if strings.TrimSpace(manifest.Problem) == "" {
		return fmt.Errorf("manifest problem is required")
	}
	if err := ValidateLanguageSelection(manifest.Languages); err != nil {
		return err
	}
	if manifest.Matt.Repository == "" || manifest.Matt.Commit == "" || manifest.Matt.Path == "" {
		return fmt.Errorf("manifest Matt dependency must include repository, commit, and path")
	}
	if !sameStrings(manifest.Hooks.Required, RequiredHookEvents) {
		return fmt.Errorf("manifest must require all five lifecycle hooks")
	}
	if !manifest.Hooks.Enabled {
		return fmt.Errorf("manifest hooks must remain enabled")
	}
	return nil
}

// ValidateLanguageSelection checks that a non-empty language set contains only
// supported profiles and does not repeat a normalized language name.
func ValidateLanguageSelection(languages []string) error {
	if len(languages) == 0 {
		return fmt.Errorf("at least one supported language is required")
	}
	seen := map[string]bool{}
	for _, language := range languages {
		language = normalizeLanguage(language)
		if !supportedLanguage(language) {
			return fmt.Errorf("unsupported language %q", language)
		}
		if seen[language] {
			return fmt.Errorf("duplicate language %q", language)
		}
		seen[language] = true
	}
	return nil
}

// ValidateGitHabits checks the schema, profile, Git refs, release settings,
// remote configuration, and supported action policy.
func ValidateGitHabits(config GitHabits) error {
	if config.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported githabits schema_version %q", config.SchemaVersion)
	}
	if config.Version < 1 {
		return fmt.Errorf("githabits version must be positive")
	}
	if config.Profile != "manual" && config.Profile != "guided" && config.Profile != "autonomous" {
		return fmt.Errorf("unsupported githabits profile %q", config.Profile)
	}
	if !safeGitRef(config.Branch) {
		return fmt.Errorf("branch is not a safe Git ref: %q", config.Branch)
	}
	if config.Versioning != "semver" {
		return fmt.Errorf("unsupported versioning policy %q", config.Versioning)
	}
	if config.CommitStyle != "conventional-commits" && config.CommitStyle != "freeform" {
		return fmt.Errorf("unsupported commit style %q", config.CommitStyle)
	}
	if config.InitialTag == "" || !safeGitRef(config.InitialTag) {
		return fmt.Errorf("initial_tag is not a safe Git ref")
	}
	if config.Remote.Provider != "github" && config.Remote.Provider != "custom" {
		return fmt.Errorf("unsupported remote provider %q", config.Remote.Provider)
	}
	if config.Remote.Visibility != "private" && config.Remote.Visibility != "public" && config.Remote.Visibility != "internal" {
		return fmt.Errorf("unsupported remote visibility %q", config.Remote.Visibility)
	}
	if !safeGitRef(config.Remote.Alias) {
		return fmt.Errorf("remote alias is not a safe Git ref: %q", config.Remote.Alias)
	}
	if config.Remote.Status != "pending" && config.Remote.Status != "configured" && config.Remote.Status != "disabled" {
		return fmt.Errorf("unsupported remote status %q", config.Remote.Status)
	}
	if config.Remote.Status == "configured" && strings.TrimSpace(config.Remote.URL) == "" {
		return fmt.Errorf("configured remote must include a URL")
	}
	return nil
}

func supportedLanguage(language string) bool {
	if language == "ts" || language == "js" {
		return true
	}
	for _, supported := range SupportedLanguages {
		if language == supported {
			return true
		}
	}
	return false
}

func normalizeLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "ts", "typescript":
		return "typescript"
	case "js", "javascript":
		return "javascript"
	case "jvm", "java":
		return "java"
	case "c#", "cs", "csharp", "dotnet":
		return "csharp"
	default:
		return strings.ToLower(strings.TrimSpace(language))
	}
}

func safeGitRef(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.HasPrefix(value, "-") && !strings.ContainsAny(value, " ~^:?*[\\\\")
}

// ScaffoldOptions configures the files and policy created for a project.
type ScaffoldOptions struct {
	ProjectName     string
	Problem         string
	Languages       []string
	Profile         string
	ActionOverrides map[GitAction]bool
	Force           bool
	Bootstrap       bool
	AllowHere       bool
}

// ScaffoldResult lists the project root and paths created or left untouched.
type ScaffoldResult struct {
	Root    string   `json:"root"`
	Created []string `json:"created"`
	Skipped []string `json:"skipped"`
}

// Assessment describes a read-only preview of paths a scaffold would change.
type Assessment struct {
	Mode      string   `json:"mode"`
	Root      string   `json:"root"`
	Existing  []string `json:"existing"`
	Proposed  []string `json:"proposed"`
	Conflicts []string `json:"conflicts"`
	ReadOnly  bool     `json:"read_only"`
}

// Projection describes the generated architecture views and their ADR count.
type Projection struct {
	ADRCount int      `json:"adr_count"`
	Paths    []string `json:"paths"`
}

// OwnershipLedger maps generated project-relative paths to their recorded hashes.
type OwnershipLedger struct {
	Version int               `json:"version"`
	Files   map[string]string `json:"files"`
}

// UninstallAssessment previews which owned files can be removed and which
// paths must be preserved as conflicts.
type UninstallAssessment struct {
	Mode      string   `json:"mode"`
	Root      string   `json:"root"`
	Removable []string `json:"removable"`
	Conflicts []string `json:"conflicts"`
	Missing   []string `json:"missing"`
	ReadOnly  bool     `json:"read_only"`
}

// UninstallResult lists files removed, preserved, or already missing after
// an approved uninstall.
type UninstallResult struct {
	Root      string   `json:"root"`
	Removed   []string `json:"removed"`
	Preserved []string `json:"preserved"`
	Missing   []string `json:"missing"`
}

// ArchitectureContract is a generated summary of the project manifest and ADRs.
type ArchitectureContract struct {
	SchemaVersion string   `yaml:"schema_version" json:"schema_version"`
	Revision      int      `yaml:"revision" json:"revision"`
	Scope         string   `yaml:"scope" json:"scope"`
	Problem       string   `yaml:"problem" json:"problem"`
	Languages     []string `yaml:"languages" json:"languages"`
	ADRFiles      []string `yaml:"adr_files" json:"adr_files"`
}

func projectPaths(root string) []string {
	return []string{
		ManifestFilename,
		GitHabitsFilename,
		ContextFilename,
		ContextMapFilename,
		"AGENTS.md",
		MattDependencyMetadataPath,
		".agents/skills/githabits/SKILL.md",
		HookConfigPath,
		SessionPath,
		OwnershipFilename,
		ContractFilename,
		ImplementationPlanFilename,
		ADRIndexFilename,
	}
}

func sortedLanguages(languages []string) []string {
	result := make([]string, 0, len(languages))
	seen := map[string]bool{}
	for _, language := range languages {
		language = normalizeLanguage(language)
		if language != "" && !seen[language] {
			seen[language] = true
			result = append(result, language)
		}
	}
	sort.Strings(result)
	return result
}

func relativePath(root, path string) string {
	value, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(value)
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]bool, len(left))
	for _, value := range left {
		seen[value] = true
	}
	for _, value := range right {
		if !seen[value] {
			return false
		}
	}
	return true
}
