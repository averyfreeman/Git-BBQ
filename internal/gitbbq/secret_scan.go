package gitbbq

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	maxArchitectureArtifacts      = 2_000
	maxArchitectureArtifactBytes  = 2 << 20
	maxArchitectureScanBytes      = 16 << 20
	maxArchitectureSecretFindings = 100
)

var architectureSecretPatterns = []struct {
	kind    string
	pattern *regexp.Regexp
}{
	{kind: "private key marker", pattern: regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----`)},
	{kind: "GitHub token", pattern: regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,})\b`)},
	{kind: "OpenAI token", pattern: regexp.MustCompile(`\bsk-(?:proj-|live-|svcacct-)?[A-Za-z0-9_-]{20,}\b`)},
	{kind: "Slack token", pattern: regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{12,}\b`)},
	{kind: "cloud access key", pattern: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{kind: "credential-bearing URL", pattern: regexp.MustCompile(`://[^\s/:@]+:[^\s/@]{8,}@`)},
}

var sensitiveAssignmentPattern = regexp.MustCompile(`(?i)["']?(?:api[_-]?key|access[_-]?token|auth[_-]?token|refresh[_-]?token|client[_-]?secret|password|passwd|secret[_-]?key|credential|bearer)["']?\s*[:=]\s*["']?([A-Za-z0-9_./+=-]{16,})`)

type architectureSecretFinding struct {
	path string
	line int
	kind string
}

func scanArchitectureSecrets(root string, includeProjections bool) error {
	return scanArchitectureSecretsWithGitHabits(root, includeProjections, true)
}

func scanArchitectureSecretsWithGitHabits(root string, includeProjections, includeGitHabits bool) error {
	paths, err := architectureArtifactPaths(root, includeProjections, includeGitHabits)
	if err != nil {
		return err
	}
	if len(paths) > maxArchitectureArtifacts {
		return fmt.Errorf("architecture secret scan stopped: artifact count exceeds limit of %d", maxArchitectureArtifacts)
	}

	totalBytes := 0
	findings := make([]architectureSecretFinding, 0)
	omittedFindings := 0
	for _, relative := range paths {
		if err := ensureArtifactParents(root, relative); err != nil {
			return err
		}
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect architecture artifact %q: %w", relative, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("architecture secret scan requires a regular file: %q", relative)
		}
		if info.Size() > maxArchitectureArtifactBytes {
			return fmt.Errorf("architecture secret scan stopped: %q exceeds the per-file limit of %d bytes", relative, maxArchitectureArtifactBytes)
		}
		remaining := maxArchitectureScanBytes - totalBytes
		if remaining <= 0 {
			if info.Size() == 0 {
				continue
			}
			return fmt.Errorf("architecture secret scan stopped: total input exceeds the %d byte limit", maxArchitectureScanBytes)
		}
		limit := maxArchitectureArtifactBytes
		if remaining < limit {
			limit = remaining
		}
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("read architecture artifact %q: %w", relative, err)
		}
		openedInfo, err := file.Stat()
		if err != nil {
			file.Close()
			return fmt.Errorf("inspect architecture artifact %q: %w", relative, err)
		}
		if !os.SameFile(info, openedInfo) {
			file.Close()
			return fmt.Errorf("architecture artifact changed during secret scan: %q", relative)
		}
		data, readErr := io.ReadAll(io.LimitReader(file, int64(limit)+1))
		closeErr := file.Close()
		if readErr != nil {
			return fmt.Errorf("read architecture artifact %q: %w", relative, readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close architecture artifact %q: %w", relative, closeErr)
		}
		if len(data) > limit {
			if limit == maxArchitectureArtifactBytes {
				return fmt.Errorf("architecture secret scan stopped: %q exceeds the per-file limit of %d bytes", relative, maxArchitectureArtifactBytes)
			}
			return fmt.Errorf("architecture secret scan stopped: total input exceeds the %d byte limit", maxArchitectureScanBytes)
		}
		totalBytes += len(data)

		for lineNumber, line := range strings.Split(string(data), "\n") {
			for _, kind := range secretKindsOnLine(line) {
				if len(findings) < maxArchitectureSecretFindings {
					findings = append(findings, architectureSecretFinding{path: relative, line: lineNumber + 1, kind: kind})
				} else {
					omittedFindings++
				}
			}
		}
	}
	if len(findings) == 0 {
		return nil
	}
	var message strings.Builder
	message.WriteString("secret-like values found in architecture artifacts:")
	for _, finding := range findings {
		message.WriteString(fmt.Sprintf("\n- %q:%d (%s)", finding.path, finding.line, finding.kind))
	}
	if omittedFindings > 0 {
		message.WriteString(fmt.Sprintf("\n- %d additional finding(s) omitted", omittedFindings))
	}
	return fmt.Errorf("%s", message.String())
}

func architectureArtifactPaths(root string, includeProjections, includeGitHabits bool) ([]string, error) {
	paths := []string{
		ManifestFilename,
		ContextFilename,
		ContextMapFilename,
		"AGENTS.md",
		MattDependencyMetadataPath,
	}
	if includeGitHabits {
		paths = append(paths, GitHabitsFilename)
	}
	if includeProjections {
		paths = append(paths, ContractFilename, ImplementationPlanFilename, ADRIndexFilename)
	}
	if err := ensureArtifactParents(root, filepath.ToSlash(filepath.Join(ADRDirectory, ".scan"))); err != nil {
		return nil, err
	}
	adrDirectory := filepath.Join(root, ADRDirectory)
	info, err := os.Lstat(adrDirectory)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("architecture secret scan requires a regular directory: %q", ADRDirectory)
		}
		entries, err := os.ReadDir(adrDirectory)
		if err != nil {
			return nil, fmt.Errorf("list architecture records: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
				paths = append(paths, filepath.ToSlash(filepath.Join(ADRDirectory, entry.Name())))
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect architecture records: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func ensureArtifactParents(root, relative string) error {
	parts := strings.Split(filepath.FromSlash(relative), string(filepath.Separator))
	current := root
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect architecture artifact parent %q: %w", relative, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("architecture secret scan requires regular directory parents: %q", relative)
		}
	}
	return nil
}

func secretKindsOnLine(line string) []string {
	kinds := make([]string, 0, 2)
	seen := make(map[string]bool)
	add := func(kind string) {
		if !seen[kind] {
			seen[kind] = true
			kinds = append(kinds, kind)
		}
	}
	for _, detector := range architectureSecretPatterns {
		for _, match := range detector.pattern.FindAllString(line, -1) {
			if !isPlaceholderSecret(match) {
				add(detector.kind)
				break
			}
		}
	}
	if matches := sensitiveAssignmentPattern.FindAllStringSubmatch(line, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 && !isPlaceholderSecret(match[1]) {
				add("credential assignment")
				break
			}
		}
	}
	return kinds
}

func isPlaceholderSecret(value string) bool {
	value = strings.ToLower(strings.Trim(value, "\"'`.,;:)}]"))
	for _, marker := range []string{"example", "placeholder", "redacted", "replace", "changeme", "change-me", "your-", "your_", "dummy", "sample", "fake"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return strings.HasPrefix(value, "${") || strings.HasPrefix(value, "<")
}
