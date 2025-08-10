package template

import "sort"

// supportedLanguageExtensions defines the output file extension for each supported language.
// If you add a new language template, also add it here.
var supportedLanguageExtensions = map[string]string{
	"python":     "py",
	"go":         "go",
	"javascript": "js",
	"java":       "java",
	"csharp":     "cs",
	"dotnet":     "cs",
	"ruby":       "rb",
	"php":        "php",
}

// getOutputFilenameForLanguage returns the output filename for a given language.
// Most languages use the convention "otel.{ext}".
// For languages with identifier constraints (e.g., Java, C#), we use "Otel.{ext}".
func getOutputFilenameForLanguage(language string) string {
	switch language {
	case "java":
		return "Otel.java"
	case "dotnet", "csharp":
		return "Otel.cs"
	default:
		if ext, ok := supportedLanguageExtensions[language]; ok {
			return "otel." + ext
		}
		return "otel.txt"
	}
}

// getSupportedLanguages returns all supported language identifiers in a stable order.
func getSupportedLanguages() []string {
	languages := make([]string, 0, len(supportedLanguageExtensions))
	for lang := range supportedLanguageExtensions {
		languages = append(languages, lang)
	}
	sort.Strings(languages)
	return languages
}
