package gitbbq

type LanguageProfile struct {
	Language      string
	DisplayName   string
	Build         string
	Test          string
	Format        string
	Documentation string
}

var languageProfiles = map[string]LanguageProfile{
	"go": {
		Language: "go", DisplayName: "Go", Build: "go build ./...", Test: "go test ./...",
		Format: "gofmt -w .", Documentation: "Go package comments and go doc",
	},
	"python": {
		Language: "python", DisplayName: "Python", Build: "project-specific", Test: "pytest",
		Format: "ruff format", Documentation: "package and module docstrings",
	},
	"typescript": {
		Language: "typescript", DisplayName: "TypeScript", Build: "npm run build", Test: "npm test",
		Format: "npm run format", Documentation: "TSDoc and project documentation",
	},
	"javascript": {
		Language: "javascript", DisplayName: "JavaScript", Build: "npm run build", Test: "npm test",
		Format: "npm run format", Documentation: "JSDoc and project documentation",
	},
	"rust": {
		Language: "rust", DisplayName: "Rust", Build: "cargo build", Test: "cargo test",
		Format: "cargo fmt", Documentation: "rustdoc",
	},
	"java": {
		Language: "java", DisplayName: "Java", Build: "mvn package", Test: "mvn test",
		Format: "mvn spotless:apply", Documentation: "Javadoc",
	},
	"csharp": {
		Language: "csharp", DisplayName: "C#", Build: "dotnet build", Test: "dotnet test",
		Format: "dotnet format", Documentation: "XML documentation comments",
	},
}

func ProfileForLanguage(language string) (LanguageProfile, bool) {
	profile, ok := languageProfiles[normalizeLanguage(language)]
	return profile, ok
}

func LanguageProfileGuidance(languages []string) []LanguageProfile {
	profiles := make([]LanguageProfile, 0, len(languages))
	for _, language := range sortedLanguages(languages) {
		if profile, ok := ProfileForLanguage(language); ok {
			profiles = append(profiles, profile)
		}
	}
	return profiles
}
