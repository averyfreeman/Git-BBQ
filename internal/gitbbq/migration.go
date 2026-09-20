package gitbbq

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	legacyarchitect "github.com/averyfreeman/git-bbq/internal/legacyarchitect"
	"gopkg.in/yaml.v3"
)

// MigrationAssessment describes the legacy scaffold and the Git BBQ paths that
// an approved migration would create or reconcile. It never writes files.
type MigrationAssessment struct {
	Mode      string   `json:"mode"`
	Root      string   `json:"root"`
	Detected  bool     `json:"detected"`
	Legacy    []string `json:"legacy"`
	Proposed  []string `json:"proposed"`
	Conflicts []string `json:"conflicts"`
	Warnings  []string `json:"warnings,omitempty"`
	ReadOnly  bool     `json:"read_only"`
}

type MigrationResult struct {
	Root         string   `json:"root"`
	Created      []string `json:"created"`
	Skipped      []string `json:"skipped"`
	Archived     []string `json:"archived"`
	MigratedADRs int      `json:"migrated_adrs"`
	Warnings     []string `json:"warnings,omitempty"`
}

type legacyScaffoldMetadata struct {
	Language string `yaml:"language"`
}

type legacyContextMetadata struct {
	Motivation struct {
		Problem string `yaml:"problem"`
	} `yaml:"motivation"`
}

type plannedMigrationADR struct {
	Relative string
	Content  []byte
}

// AssessMigration inventories the legacy AI Software Architect scaffold and
// reports the target Git BBQ paths without changing the repository.
func AssessMigration(root string) (MigrationAssessment, error) {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		return MigrationAssessment{}, err
	}
	if !info.IsDir() {
		return MigrationAssessment{}, fmt.Errorf("migration root is not a directory: %s", root)
	}
	existing, err := SnapshotFiles(root)
	if err != nil {
		return MigrationAssessment{}, err
	}
	legacy := make([]string, 0)
	existingSet := make(map[string]bool, len(existing))
	for _, relative := range existing {
		existingSet[relative] = true
		if isLegacyScaffoldPath(relative) {
			legacy = append(legacy, relative)
		}
	}
	sort.Strings(legacy)
	proposed := append([]string(nil), projectPaths(root)...)
	sort.Strings(proposed)
	conflicts := make([]string, 0)
	for _, relative := range proposed {
		if existingSet[relative] {
			conflicts = append(conflicts, relative)
		}
	}
	warnings := []string{
		"migration is read-only until the user reviews this inventory and gives explicit approval for the apply step",
		"legacy .ai-architect artifacts remain in place; converted ADRs will be additive under docs/adr",
	}
	if len(legacy) == 0 {
		warnings = append(warnings, "no legacy AI Software Architect scaffold was detected")
	}
	return MigrationAssessment{
		Mode:      "migration-assessment",
		Root:      root,
		Detected:  len(legacy) > 0,
		Legacy:    legacy,
		Proposed:  proposed,
		Conflicts: conflicts,
		Warnings:  warnings,
		ReadOnly:  true,
	}, nil
}

// Migrate applies the approved, additive legacy migration. It preserves the
// legacy artifact tree and validates the resulting Git BBQ project before it
// returns.
func Migrate(root string, force bool) (MigrationResult, error) {
	root = filepath.Clean(root)
	if root == "." {
		root, _ = os.Getwd()
	}
	assessment, err := AssessMigration(root)
	if err != nil {
		return MigrationResult{}, err
	}
	if !assessment.Detected {
		return MigrationResult{}, fmt.Errorf("no legacy AI Software Architect scaffold detected")
	}
	if _, err := ReadManifest(root); err == nil {
		return MigrationResult{}, fmt.Errorf("Git BBQ manifest already exists; migration is not needed")
	}
	language, problem, err := legacyProjectInputs(root)
	if err != nil {
		return MigrationResult{}, err
	}
	plannedADRs, err := planLegacyADRs(root)
	if err != nil {
		return MigrationResult{}, err
	}
	archived, archivePath, archiveCreated, err := archiveLegacyGitHabits(root, force)
	if err != nil {
		return MigrationResult{}, err
	}
	result := MigrationResult{Root: root, Created: []string{}, Skipped: []string{}, Archived: archived, Warnings: []string{
		"legacy .ai-architect artifacts were preserved; review migrated ADRs before accepting them",
	}}
	scaffold, err := ScaffoldProject(root, ScaffoldOptions{
		ProjectName: filepath.Base(root),
		Problem:     problem,
		Languages:   []string{language},
		Profile:     "guided",
		AllowHere:   true,
	})
	if err != nil {
		return MigrationResult{}, err
	}
	result.Created = append(result.Created, scaffold.Created...)
	result.Skipped = append(result.Skipped, scaffold.Skipped...)
	if archivePath != "" {
		config, err := GitHabitsForProfile("guided")
		if err != nil {
			return MigrationResult{}, err
		}
		data, err := marshalYAML(config)
		if err != nil {
			return MigrationResult{}, err
		}
		created, err := writeGenerated(root, GitHabitsFilename, data, true)
		if err != nil {
			return MigrationResult{}, err
		}
		if created {
			result.Created = append(result.Created, GitHabitsFilename)
		}
		if archiveCreated {
			result.Created = append(result.Created, filepath.ToSlash(filepath.Join(".gitbbq", "migration", "legacy", GitHabitsFilename)))
		}
	}
	for _, planned := range plannedADRs {
		created, err := writeGenerated(root, planned.Relative, planned.Content, false)
		if err != nil {
			return MigrationResult{}, err
		}
		if created {
			result.Created = append(result.Created, planned.Relative)
			result.MigratedADRs++
		}
	}
	projection, projectionCreated, err := projectWithOptions(root, false)
	if err != nil {
		return MigrationResult{}, fmt.Errorf("write migration projections: %w", err)
	}
	result.Created = append(result.Created, projectionCreated...)
	for _, relative := range projection.Paths {
		created := false
		for _, generated := range projectionCreated {
			if generated == relative {
				created = true
				break
			}
		}
		if !created {
			result.Skipped = append(result.Skipped, relative)
		}
	}
	if err := recordGeneratedOwnership(root, result.Created); err != nil {
		return MigrationResult{}, fmt.Errorf("record migration ownership: %w", err)
	}
	if err := ValidateProject(root); err != nil {
		return MigrationResult{}, fmt.Errorf("validate migrated project: %w", err)
	}
	sort.Strings(result.Created)
	sort.Strings(result.Skipped)
	sort.Strings(result.Archived)
	return result, nil
}

func legacyProjectInputs(root string) (string, string, error) {
	language := ""
	configPath := filepath.Join(root, ".adr-scaffold.yaml")
	if data, err := os.ReadFile(configPath); err == nil {
		var metadata legacyScaffoldMetadata
		if err := yaml.Unmarshal(data, &metadata); err != nil {
			return "", "", fmt.Errorf("parse %s: %w", configPath, err)
		}
		language = normalizeLanguage(metadata.Language)
	}
	if language == "" || language == "auto" {
		return "", "", fmt.Errorf("legacy scaffold language is missing; choose a supported language before migration")
	}
	if !supportedLanguage(language) {
		return "", "", fmt.Errorf("legacy scaffold language %q is not supported by Git BBQ", language)
	}
	problem := "Migrate the existing AI Software Architect project to Git BBQ."
	contextPath := filepath.Join(root, legacyarchitect.ContextFilename)
	if data, err := os.ReadFile(contextPath); err == nil {
		front, _, splitErr := legacyFrontmatter(string(data))
		if splitErr != nil {
			return "", "", fmt.Errorf("parse %s: %w", contextPath, splitErr)
		}
		var context legacyContextMetadata
		if err := yaml.Unmarshal([]byte(front), &context); err != nil {
			return "", "", fmt.Errorf("parse %s frontmatter: %w", contextPath, err)
		}
		if strings.TrimSpace(context.Motivation.Problem) != "" {
			problem = strings.TrimSpace(context.Motivation.Problem)
		}
	}
	return language, problem, nil
}

func planLegacyADRs(root string) ([]plannedMigrationADR, error) {
	records, invalid, err := legacyarchitect.ListDecisions(root, "")
	if err != nil {
		return nil, fmt.Errorf("read legacy ADRs: %w", err)
	}
	if len(invalid) > 0 {
		return nil, fmt.Errorf("legacy ADRs require repair before migration: %s", strings.Join(invalid, ", "))
	}
	plans := make([]plannedMigrationADR, 0, len(records))
	number, err := nextADRNumber(root)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		decision := record.Artifact.Decision
		why := strings.TrimSpace(decision.Motivation.WhyThisSolution)
		if why == "" {
			why = "Preserve the validated legacy decision while adopting the Matt-native ADR path."
		}
		status := decision.Status
		if status == "rejected" || status == "superseded" {
			status = "deprecated"
		}
		if status != "accepted" && status != "deprecated" {
			status = "proposed"
		}
		input := ADRInput{Title: decision.Title, Context: decision.Context, Decision: decision.Decision, Why: why, Status: status}
		filename := fmt.Sprintf("%04d-%s.md", number, slugify(input.Title))
		if strings.HasSuffix(filename, "-.md") {
			return nil, fmt.Errorf("legacy ADR %s has no migratable title", record.Filename)
		}
		relative := filepath.ToSlash(filepath.Join(ADRDirectory, filename))
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(relative))); err == nil {
			return nil, fmt.Errorf("migration target already exists: %s", relative)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		plans = append(plans, plannedMigrationADR{Relative: relative, Content: []byte(renderADR(input))})
		number++
	}
	return plans, nil
}

func archiveLegacyGitHabits(root string, force bool) ([]string, string, bool, error) {
	path := filepath.Join(root, GitHabitsFilename)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, err
	}
	if _, validErr := ReadGitHabits(root); validErr == nil {
		return nil, "", false, nil
	}
	if !force {
		return nil, "", false, fmt.Errorf("legacy %s must be archived with --force before migration", GitHabitsFilename)
	}
	relative := filepath.ToSlash(filepath.Join(".gitbbq", "migration", "legacy", GitHabitsFilename))
	created, err := writeGenerated(root, relative, data, false)
	if err != nil {
		return nil, "", false, err
	}
	return []string{relative}, relative, created, nil
}

func legacyFrontmatter(content string) (string, string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return "", content, fmt.Errorf("frontmatter is required")
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		return "", "", fmt.Errorf("frontmatter is not terminated")
	}
	end += 4
	return content[4:end], content[end+5:], nil
}

func isLegacyScaffoldPath(relative string) bool {
	return relative == ".adr-scaffold.yaml" || relative == ".ai-architect" || strings.HasPrefix(relative, ".ai-architect/")
}
