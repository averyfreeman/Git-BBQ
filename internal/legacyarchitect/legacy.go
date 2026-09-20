// Package legacyarchitect contains the bounded reader needed to migrate
// legacy AI Software Architect decision files without importing that project.
package legacyarchitect

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ContextFilename = ".ai-architect/project-context.md"
	DecisionsDir    = ".ai-architect/decisions"
	maxFileBytes    = 500_000
	maxDecisions    = 200
	maxYAMLDepth    = 50
)

var decisionFilenamePattern = regexp.MustCompile(`^ADR-[0-9]{3}(?:-[a-z0-9]+(?:-[a-z0-9]+)*)?\.md$`)

type Motivation struct {
	WhyBuilding     string `yaml:"why_building"`
	Problem         string `yaml:"problem"`
	Audience        string `yaml:"audience"`
	Alternatives    string `yaml:"alternatives"`
	WhyThisSolution string `yaml:"why_this_solution"`
	Source          string `yaml:"source,omitempty"`
}

type DecisionOption struct {
	ID        string   `yaml:"id"`
	Name      string   `yaml:"name"`
	FitScore  int      `yaml:"fit_score"`
	Benefits  []string `yaml:"benefits"`
	Drawbacks []string `yaml:"drawbacks"`
	Risks     []string `yaml:"risks"`
	Evidence  []string `yaml:"evidence"`
	Outcome   string   `yaml:"outcome"`
}

type ActionContract struct {
	GoodOutcomes     []string `yaml:"good_outcomes"`
	BadOutcomes      []string `yaml:"bad_outcomes"`
	CorrectExample   string   `yaml:"correct_example"`
	IncorrectExample string   `yaml:"incorrect_example"`
}

type Decision struct {
	ID                   string           `yaml:"id"`
	Title                string           `yaml:"title"`
	Date                 string           `yaml:"date"`
	Status               string           `yaml:"status"`
	Motivation           Motivation       `yaml:"motivation"`
	Context              string           `yaml:"context"`
	Drivers              []string         `yaml:"drivers"`
	ConsideredOptionIDs  []string         `yaml:"considered_option_ids"`
	SelectedOptionID     *string          `yaml:"selected_option_id"`
	Decision             string           `yaml:"decision"`
	PositiveConsequences []string         `yaml:"positive_consequences"`
	NegativeConsequences []string         `yaml:"negative_consequences"`
	Assumptions          []string         `yaml:"assumptions"`
	ValidationCriteria   []string         `yaml:"validation_criteria"`
	Supersedes           []string         `yaml:"supersedes"`
	DecisionMatrix       []DecisionOption `yaml:"decision_matrix"`
	ActionContract       ActionContract   `yaml:"action_contract"`
}

type DecisionArtifact struct {
	SchemaVersion string   `yaml:"schema_version"`
	Revision      int      `yaml:"revision"`
	Decision      Decision `yaml:"decision"`
}

type DecisionRecord struct {
	Artifact DecisionArtifact
	Body     string
	Filename string
	Path     string
}

// ListDecisions reads legacy decisions without writing or executing project code.
func ListDecisions(root, status string) ([]DecisionRecord, []string, error) {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("legacy migration root is not a directory: %s", root)
	}

	directory := filepath.Join(root, filepath.FromSlash(DecisionsDir))
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return []DecisionRecord{}, []string{}, nil
	}
	if err != nil {
		return nil, nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !decisionFilenamePattern.MatchString(entry.Name()) {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Slice(files, func(i, j int) bool {
		return decisionNumber(files[i]) < decisionNumber(files[j])
	})
	if len(files) > maxDecisions {
		files = files[:maxDecisions]
	}

	records := make([]DecisionRecord, 0, len(files))
	invalid := make([]string, 0)
	for _, filename := range files {
		path := filepath.Join(directory, filename)
		record, parseErr := loadDecision(path)
		if parseErr != nil {
			invalid = append(invalid, filepath.ToSlash(path))
			continue
		}
		if status == "" || strings.EqualFold(record.Artifact.Decision.Status, status) {
			records = append(records, *record)
		}
	}
	return records, invalid, nil
}

func loadDecision(path string) (*DecisionRecord, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return nil, fmt.Errorf("legacy decision file is not a bounded regular file: %s", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	body, artifact, err := parseFrontmatter(string(content))
	if err != nil {
		return nil, err
	}
	return &DecisionRecord{
		Artifact: artifact,
		Body:     body,
		Filename: filepath.Base(path),
		Path:     path,
	}, nil
}

func parseFrontmatter(content string) (string, DecisionArtifact, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.SplitAfter(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", DecisionArtifact{}, fmt.Errorf("legacy decision frontmatter is required")
	}
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) != "---" {
			continue
		}
		front := strings.Join(lines[1:index], "")
		body := strings.Join(lines[index+1:], "")
		var artifact DecisionArtifact
		if err := unmarshalSafe([]byte(front), &artifact); err != nil {
			return "", DecisionArtifact{}, fmt.Errorf("parse legacy decision frontmatter: %w", err)
		}
		return body, artifact, nil
	}
	return "", DecisionArtifact{}, fmt.Errorf("legacy decision frontmatter is not terminated")
}

func unmarshalSafe(data []byte, target any) error {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return err
	}
	if len(node.Content) == 0 {
		return fmt.Errorf("YAML document is empty")
	}
	if err := validateYAMLNode(node.Content[0], 0); err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	return decoder.Decode(target)
}

func validateYAMLNode(node *yaml.Node, depth int) error {
	if depth > maxYAMLDepth {
		return fmt.Errorf("YAML nesting exceeds %d", maxYAMLDepth)
	}
	if node.Kind == yaml.AliasNode {
		return fmt.Errorf("YAML aliases are not allowed")
	}
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]struct{}, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index]
			if key.Kind != yaml.ScalarNode {
				return fmt.Errorf("YAML mapping keys must be scalar values")
			}
			if _, exists := seen[key.Value]; exists {
				return fmt.Errorf("duplicate YAML key %q", key.Value)
			}
			seen[key.Value] = struct{}{}
			if err := validateYAMLNode(key, depth+1); err != nil {
				return err
			}
			if err := validateYAMLNode(node.Content[index+1], depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	for _, child := range node.Content {
		if err := validateYAMLNode(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func decisionNumber(filename string) int {
	if len(filename) < 7 {
		return 0
	}
	number, _ := strconv.Atoi(filename[4:7])
	return number
}
