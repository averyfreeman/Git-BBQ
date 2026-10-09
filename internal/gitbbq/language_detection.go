package gitbbq

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxLanguageDetectionFiles = 100_000
	maxLanguageEvidence       = 12
)

var excludedLanguageDirectories = map[string]struct{}{
	".agents": {}, ".cache": {}, ".git": {}, ".gitbbq": {}, ".gradle": {},
	".mypy_cache": {}, ".next": {}, ".nuxt": {}, ".pytest_cache": {},
	".tox": {}, ".turbo": {}, ".venv": {}, "bin": {}, "bower_components": {},
	"build": {}, "carthage": {}, "coverage": {}, "dist": {}, "generated": {},
	"node_modules": {}, "obj": {}, "out": {}, "pods": {}, "target": {},
	"third-party": {}, "third_party": {}, "vendor": {}, "venv": {}, "__pycache__": {},
}

var errLanguageDetectionLimit = errors.New("language detection file limit reached")

// LanguageEvidence identifies a project-relative path that supports a
// language candidate. Kind is "manifest" or "source".
type LanguageEvidence struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// LanguageCandidate groups the evidence found for one supported language.
// EvidenceOmitted counts additional matches beyond the returned evidence cap.
type LanguageCandidate struct {
	Language        string             `json:"language"`
	Evidence        []LanguageEvidence `json:"evidence"`
	EvidenceOmitted int                `json:"evidence_omitted,omitempty"`
}

// LanguageDetection contains the cleaned scan root and detected language
// candidates. Truncated is true when the scanner reached its file limit.
type LanguageDetection struct {
	Root       string              `json:"root"`
	Candidates []LanguageCandidate `json:"candidates"`
	Truncated  bool                `json:"truncated,omitempty"`
}

// DetectLanguages scans project markers and first-party source filenames. It
// never reads source contents or writes files, and returns every supported
// language with the paths that support the candidate.
func DetectLanguages(root string) (LanguageDetection, error) {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		return LanguageDetection{}, err
	}
	if !info.IsDir() {
		return LanguageDetection{}, fmt.Errorf("language detection root is not a directory: %s", root)
	}

	result := LanguageDetection{Root: root}
	byLanguage := make(map[string]*LanguageCandidate, len(SupportedLanguages))
	type languageSource struct {
		language string
		path     string
	}
	sourceFiles := make([]languageSource, 0)
	add := func(language, relative, kind string) {
		if kind == "source" {
			sourceFiles = append(sourceFiles, languageSource{language: language, path: relative})
		}
		candidate := byLanguage[language]
		if candidate == nil {
			candidate = &LanguageCandidate{Language: language, Evidence: []LanguageEvidence{}}
			byLanguage[language] = candidate
		}
		for _, evidence := range candidate.Evidence {
			if evidence.Path == relative && evidence.Kind == kind {
				return
			}
		}
		if len(candidate.Evidence) < maxLanguageEvidence {
			candidate.Evidence = append(candidate.Evidence, LanguageEvidence{Path: relative, Kind: kind})
		} else {
			candidate.EvidenceOmitted++
		}
	}

	files := 0
	packageManifests := make([]string, 0)
	typescriptConfigPaths := make([]string, 0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.IsDir() {
			if _, excluded := excludedLanguageDirectories[strings.ToLower(entry.Name())]; excluded {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return nil
		}
		files++
		if files > maxLanguageDetectionFiles {
			return errLanguageDetectionLimit
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		relative = filepath.ToSlash(relative)
		base := strings.ToLower(filepath.Base(relative))
		ext := strings.ToLower(filepath.Ext(base))

		switch {
		case base == "go.mod" || base == "go.work" || base == "go.sum":
			add("go", relative, "manifest")
		case strings.HasPrefix(base, "tsconfig") && strings.HasSuffix(base, ".json"):
			add("typescript", relative, "manifest")
			typescriptConfigPaths = append(typescriptConfigPaths, relative)
		case base == "package.json":
			packageManifests = append(packageManifests, relative)
		case base == "pyproject.toml" || base == "requirements.txt" || base == "setup.py" || base == "setup.cfg" || base == "pipfile":
			add("python", relative, "manifest")
		case base == "cargo.toml" || base == "cargo.lock":
			add("rust", relative, "manifest")
		case base == "pom.xml" || base == "build.gradle" || base == "build.gradle.kts" || base == "settings.gradle" || base == "settings.gradle.kts":
			add("java", relative, "manifest")
		case strings.HasSuffix(base, ".csproj") || strings.HasSuffix(base, ".sln"):
			add("csharp", relative, "manifest")
		}

		if isGeneratedLanguageSource(base) {
			return nil
		}
		switch ext {
		case ".go":
			add("go", relative, "source")
		case ".py":
			add("python", relative, "source")
		case ".ts", ".tsx":
			add("typescript", relative, "source")
		case ".js", ".jsx", ".mjs", ".cjs":
			add("javascript", relative, "source")
		case ".rs":
			add("rust", relative, "source")
		case ".java":
			add("java", relative, "source")
		case ".cs":
			add("csharp", relative, "source")
		}
		return nil
	})
	if errors.Is(err, errLanguageDetectionLimit) {
		result.Truncated = true
	} else if err != nil {
		return LanguageDetection{}, fmt.Errorf("scan language evidence: %w", err)
	}

	packageRoots := make([]string, 0, len(packageManifests))
	packageRootSet := make(map[string]bool, len(packageManifests))
	for _, path := range packageManifests {
		packageRoot := filepath.ToSlash(filepath.Dir(path))
		packageRoots = append(packageRoots, packageRoot)
		packageRootSet[packageRoot] = true
	}
	packageEvidence := make(map[string]map[string]bool, len(packageRoots))
	for _, source := range sourceFiles {
		packageRoot := nearestPackageRoot(source.path, packageRootSet)
		if packageRoot == "" {
			continue
		}
		if packageEvidence[packageRoot] == nil {
			packageEvidence[packageRoot] = map[string]bool{}
		}
		packageEvidence[packageRoot][source.language] = true
	}
	for _, configPath := range typescriptConfigPaths {
		packageRoot := nearestPackageRoot(configPath, packageRootSet)
		if packageRoot == "" {
			continue
		}
		if packageEvidence[packageRoot] == nil {
			packageEvidence[packageRoot] = map[string]bool{}
		}
		packageEvidence[packageRoot]["typescript"] = true
	}
	for i, path := range packageManifests {
		packageRoot := packageRoots[i]
		hasJavaScript := packageEvidence[packageRoot]["javascript"]
		hasTypeScript := packageEvidence[packageRoot]["typescript"]
		if hasJavaScript {
			add("javascript", path, "manifest")
		}
		if hasTypeScript {
			add("typescript", path, "manifest")
		}
		if !hasJavaScript && !hasTypeScript {
			add("javascript", path, "manifest")
		}
	}

	for _, language := range SupportedLanguages {
		if candidate := byLanguage[language]; candidate != nil {
			sort.Slice(candidate.Evidence, func(i, j int) bool {
				if candidate.Evidence[i].Path == candidate.Evidence[j].Path {
					return candidate.Evidence[i].Kind < candidate.Evidence[j].Kind
				}
				return candidate.Evidence[i].Path < candidate.Evidence[j].Path
			})
			result.Candidates = append(result.Candidates, *candidate)
		}
	}
	return result, nil
}

func nearestPackageRoot(path string, packageRoots map[string]bool) string {
	directory := filepath.ToSlash(filepath.Dir(path))
	for {
		if packageRoots[directory] {
			return directory
		}
		if directory == "." || directory == "" {
			return ""
		}
		directory = filepath.ToSlash(filepath.Dir(directory))
	}
}

func isGeneratedLanguageSource(base string) bool {
	return strings.HasSuffix(base, ".min.js") ||
		strings.HasSuffix(base, ".min.ts") ||
		strings.HasSuffix(base, ".generated.go") ||
		strings.HasSuffix(base, "_generated.go") ||
		strings.HasSuffix(base, ".pb.go") ||
		strings.HasSuffix(base, ".generated.py") ||
		strings.HasSuffix(base, ".generated.ts") ||
		strings.HasSuffix(base, ".generated.js") ||
		strings.HasSuffix(base, ".generated.cs") ||
		strings.HasSuffix(base, ".g.cs") ||
		strings.HasSuffix(base, ".designer.cs")
}

// CandidateLanguages returns the normalized language names in detection order.
func (d LanguageDetection) CandidateLanguages() []string {
	languages := make([]string, 0, len(d.Candidates))
	for _, candidate := range d.Candidates {
		languages = append(languages, candidate.Language)
	}
	return languages
}
