package gitbbq

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ADRInput struct {
	Title    string
	Context  string
	Decision string
	Why      string
	Status   string
}

type ADRRecord struct {
	Number   int    `json:"number"`
	Title    string `json:"title"`
	Status   string `json:"status,omitempty"`
	Filename string `json:"filename"`
	Path     string `json:"path"`
	Body     string `json:"body"`
}

type ADRIndex struct {
	SchemaVersion string      `json:"schema_version"`
	GeneratedAt   time.Time   `json:"generated_at"`
	DecisionCount int         `json:"decision_count"`
	Decisions     []ADRRecord `json:"decisions"`
}

var adrFilenamePattern = regexp.MustCompile(`^(\d{4})-([a-z0-9]+(?:-[a-z0-9]+)*)\.md$`)
var supersededStatusPattern = regexp.MustCompile(`^superseded by ADR-\d{4}$`)

func CreateADR(root string, input ADRInput) (ADRRecord, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Context = strings.TrimSpace(input.Context)
	input.Decision = strings.TrimSpace(input.Decision)
	input.Why = strings.TrimSpace(input.Why)
	input.Status = strings.TrimSpace(input.Status)
	if input.Title == "" || input.Context == "" || input.Decision == "" || input.Why == "" {
		return ADRRecord{}, fmt.Errorf("ADR title, context, decision, and why are required")
	}
	if input.Status == "" {
		input.Status = "proposed"
	}
	if !validADRStatus(input.Status) {
		return ADRRecord{}, fmt.Errorf("invalid ADR status %q", input.Status)
	}
	if err := os.MkdirAll(filepath.Join(root, ADRDirectory), 0o755); err != nil {
		return ADRRecord{}, err
	}
	number, err := nextADRNumber(root)
	if err != nil {
		return ADRRecord{}, err
	}
	slug := slugify(input.Title)
	if slug == "" {
		return ADRRecord{}, fmt.Errorf("ADR title must contain at least one letter or digit")
	}
	filename := fmt.Sprintf("%04d-%s.md", number, slug)
	path := filepath.Join(root, ADRDirectory, filename)
	if _, err := os.Stat(path); err == nil {
		return ADRRecord{}, fmt.Errorf("ADR already exists: %s", filename)
	} else if !os.IsNotExist(err) {
		return ADRRecord{}, err
	}
	for _, existing := range listADRPaths(root) {
		parsed, parseErr := ParseADR(existing)
		if parseErr != nil {
			return ADRRecord{}, parseErr
		}
		if strings.EqualFold(parsed.Title, input.Title) {
			return ADRRecord{}, fmt.Errorf("ADR with title %q already exists", input.Title)
		}
	}
	body := renderADR(input)
	relative := filepath.ToSlash(filepath.Join(ADRDirectory, filename))
	if _, err := writeGenerated(root, relative, []byte(body), false); err != nil {
		return ADRRecord{}, err
	}
	if err := recordExistingOwnership(root, []string{relative}); err != nil {
		return ADRRecord{}, fmt.Errorf("record ADR ownership: %w", err)
	}
	return ParseADR(path)
}

func ParseADR(path string) (ADRRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ADRRecord{}, err
	}
	filename := filepath.Base(path)
	matches := adrFilenamePattern.FindStringSubmatch(filename)
	if matches == nil {
		return ADRRecord{}, fmt.Errorf("ADR filename must be NNNN-slug.md: %s", filename)
	}
	number, err := strconv.Atoi(matches[1])
	if err != nil {
		return ADRRecord{}, err
	}
	status, markdown, err := splitFrontmatter(string(data))
	if err != nil {
		return ADRRecord{}, err
	}
	title, body, err := parseTitleAndBody(markdown)
	if err != nil {
		return ADRRecord{}, fmt.Errorf("%s: %w", filename, err)
	}
	record := ADRRecord{Number: number, Title: title, Status: status, Filename: filename, Path: path, Body: string(data)}
	if err := validateADRRecord(record, body); err != nil {
		return ADRRecord{}, err
	}
	return record, nil
}

func IndexADRs(root string) (ADRIndex, error) {
	index, created, err := indexADRs(root, true)
	if err != nil {
		return ADRIndex{}, err
	}
	if created {
		if err := recordExistingOwnership(root, []string{ADRIndexFilename}); err != nil {
			return ADRIndex{}, fmt.Errorf("record ADR index ownership: %w", err)
		}
	}
	return index, err
}

func indexADRs(root string, force bool) (ADRIndex, bool, error) {
	paths := listADRPaths(root)
	records := make([]ADRRecord, 0, len(paths))
	numbers := make(map[int]bool, len(paths))
	for _, path := range paths {
		record, err := ParseADR(path)
		if err != nil {
			return ADRIndex{}, false, err
		}
		if numbers[record.Number] {
			return ADRIndex{}, false, fmt.Errorf("duplicate ADR number %04d", record.Number)
		}
		numbers[record.Number] = true
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Number < records[j].Number })
	index := ADRIndex{SchemaVersion: SchemaVersion, GeneratedAt: time.Now().UTC(), DecisionCount: len(records), Decisions: records}
	if _, err := os.Stat(filepath.Join(root, ADRDirectory)); os.IsNotExist(err) {
		return index, false, nil
	} else if err != nil {
		return ADRIndex{}, false, err
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return ADRIndex{}, false, err
	}
	data = append(data, '\n')
	created, err := writeGenerated(root, ADRIndexFilename, data, force)
	if err != nil {
		return ADRIndex{}, false, err
	}
	return index, created, nil
}

func listADRPaths(root string) []string {
	entries, err := os.ReadDir(filepath.Join(root, ADRDirectory))
	if err != nil {
		return nil
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !adrFilenamePattern.MatchString(entry.Name()) {
			continue
		}
		paths = append(paths, filepath.Join(root, ADRDirectory, entry.Name()))
	}
	sort.Strings(paths)
	return paths
}

func nextADRNumber(root string) (int, error) {
	max := 0
	for _, path := range listADRPaths(root) {
		matches := adrFilenamePattern.FindStringSubmatch(filepath.Base(path))
		if matches == nil {
			continue
		}
		number, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, err
		}
		if number > max {
			max = number
		}
	}
	return max + 1, nil
}

func renderADR(input ADRInput) string {
	var builder strings.Builder
	if input.Status != "" {
		builder.WriteString("---\nstatus: ")
		builder.WriteString(input.Status)
		builder.WriteString("\n---\n\n")
	}
	builder.WriteString("# ")
	builder.WriteString(input.Title)
	builder.WriteString("\n\n")
	builder.WriteString(sentence(input.Context))
	builder.WriteString(" We decided: ")
	builder.WriteString(sentence(input.Decision))
	builder.WriteString(" The reason is: ")
	builder.WriteString(sentence(input.Why))
	builder.WriteString("\n")
	return builder.String()
}

func splitFrontmatter(content string) (string, string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return "", content, nil
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		return "", "", fmt.Errorf("unterminated ADR frontmatter")
	}
	end += 4
	var values map[string]string
	if err := yaml.Unmarshal([]byte(content[4:end]), &values); err != nil {
		return "", "", fmt.Errorf("parse ADR frontmatter: %w", err)
	}
	for key := range values {
		if key != "status" {
			return "", "", fmt.Errorf("unsupported ADR frontmatter field %q", key)
		}
	}
	status := values["status"]
	return status, content[end+5:], nil
}

func parseTitleAndBody(markdown string) (string, string, error) {
	lines := strings.Split(strings.TrimSpace(markdown), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "# ") {
		return "", "", fmt.Errorf("ADR must start with a level-one title")
	}
	title := strings.TrimSpace(strings.TrimPrefix(lines[0], "# "))
	if title == "" {
		return "", "", fmt.Errorf("ADR title is empty")
	}
	body := strings.TrimSpace(strings.Join(lines[1:], "\n"))
	if body == "" {
		return "", "", fmt.Errorf("ADR rationale is empty")
	}
	return title, body, nil
}

func validateADRRecord(record ADRRecord, body string) error {
	if record.Number < 1 || record.Number > 9999 {
		return fmt.Errorf("ADR number is out of range: %d", record.Number)
	}
	if record.Status != "" && !validADRStatus(record.Status) {
		return fmt.Errorf("%s: invalid ADR status %q", record.Filename, record.Status)
	}
	if strings.Contains(body, "schema_version:") || strings.Contains(body, "decision_matrix:") {
		return fmt.Errorf("%s: native structured ADR metadata is not permitted", record.Filename)
	}
	return nil
}

func validADRStatus(status string) bool {
	return status == "proposed" || status == "accepted" || status == "deprecated" || supersededStatusPattern.MatchString(status)
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastHyphen := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen && builder.Len() > 0 {
			builder.WriteByte('-')
			lastHyphen = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func sentence(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasSuffix(value, ".") || strings.HasSuffix(value, "!") || strings.HasSuffix(value, "?") {
		return value
	}
	return value + "."
}
