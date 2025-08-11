package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/domain"
)

// RenderAnalysis returns a nicely formatted, styled string for the analysis output.
func RenderAnalysis(analysis *detector.Analysis, detailed bool) string {
	if analysis == nil {
		return ""
	}

	// Aggregate data
	var allIssues []domain.Issue
	var allLibraries []domain.Library
	var allPackages []domain.Package
	var allInstrumentations []domain.InstrumentationInfo
	detectedLanguages := make(map[string]bool)

	for _, dirAnalysis := range analysis.DirectoryAnalyses {
		allIssues = append(allIssues, dirAnalysis.Issues...)
		allLibraries = append(allLibraries, dirAnalysis.Libraries...)
		allPackages = append(allPackages, dirAnalysis.Packages...)
		allInstrumentations = append(allInstrumentations, dirAnalysis.AvailableInstrumentations...)
		if dirAnalysis.Language != "" {
			detectedLanguages[dirAnalysis.Language] = true
		}
	}

	var languageSlice []string
	for lang := range detectedLanguages {
		languageSlice = append(languageSlice, lang)
	}
	sort.Strings(languageSlice)

	// Header
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", "📊 OpenTelemetry Analysis Results")
	b.WriteString(strings.Repeat("=", 32))
	b.WriteString("\n\n")

	// Summary block
	summary := []string{
		fmt.Sprintf("📂 Project Path: %s", analysis.RootPath),
		fmt.Sprintf("🗣️  Languages Detected: %v", languageSlice),
		fmt.Sprintf("📦 OpenTelemetry Libraries: %d", len(allLibraries)),
		fmt.Sprintf("📥 All Packages: %d", len(allPackages)),
		fmt.Sprintf("🔧 Available Instrumentations: %d", len(allInstrumentations)),
		fmt.Sprintf("📁 Directories Analyzed: %d", len(analysis.DirectoryAnalyses)),
		fmt.Sprintf("⚠️  Issues Found: %d", len(allIssues)),
	}
	b.WriteString(strings.Join(summary, "\n"))
	b.WriteString("\n\n")

	// Monorepo overview
	if len(analysis.DirectoryAnalyses) > 1 {
		b.WriteString("📚 Monorepo Overview:\n")
		b.WriteString(strings.Repeat("-", 20))
		b.WriteString("\n")

		// Sort directories by number of issues (desc), then by name
		type dirSummary struct {
			name          string
			language      string
			libraries     int
			packages      int
			instrumenters int
			issues        int
		}
		var summaries []dirSummary
		for directory, dirAnalysis := range analysis.DirectoryAnalyses {
			summaries = append(summaries, dirSummary{
				name:          directory,
				language:      dirAnalysis.Language,
				libraries:     len(dirAnalysis.Libraries),
				packages:      len(dirAnalysis.Packages),
				instrumenters: len(dirAnalysis.AvailableInstrumentations),
				issues:        len(dirAnalysis.Issues),
			})
		}
		sort.Slice(summaries, func(i, j int) bool {
			if summaries[i].issues == summaries[j].issues {
				return summaries[i].name < summaries[j].name
			}
			return summaries[i].issues > summaries[j].issues
		})

		for _, s := range summaries {
			fmt.Fprintf(&b, "  📂 %s (%s)\n", s.name, s.language)
			fmt.Fprintf(&b, "    %s Libraries: %d, %s Packages: %d, %s Instrumentations: %d, %s Issues: %d\n",
				"📦", s.libraries,
				"📥", s.packages,
				"🔧", s.instrumenters,
				"⚠️", s.issues,
			)
		}
		b.WriteString("\n")
	}

	// Libraries (deduplicated)
	if len(allLibraries) > 0 {
		b.WriteString("📦 OpenTelemetry Libraries Found:\n")
		b.WriteString(strings.Repeat("-", 33))
		b.WriteString("\n")
		// Build unique set by name+language to avoid noisy duplicates
		type libKey struct{ name, language string }
		uniq := make(map[libKey]domain.Library)
		for _, lib := range allLibraries {
			key := libKey{name: lib.Name, language: lib.Language}
			if _, exists := uniq[key]; !exists {
				uniq[key] = lib
			}
		}
		// Stable ordering by language then name
		var keys []libKey
		for k := range uniq {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].language == keys[j].language {
				return keys[i].name < keys[j].name
			}
			return keys[i].language < keys[j].language
		})
		for _, k := range keys {
			lib := uniq[k]
			if lib.Version != "" {
				fmt.Fprintf(&b, "  • %s (%s) - %s\n", lib.Name, lib.Version, lib.Language)
			} else {
				fmt.Fprintf(&b, "  • %s - %s\n", lib.Name, lib.Language)
			}
			if detailed && lib.PackageFile != "" {
				fmt.Fprintf(&b, "    %s %s\n", "📄 Found in:", lib.PackageFile)
			}
		}
		b.WriteString("\n")
	}

	// Available Instrumentations
	if len(allInstrumentations) > 0 {
		b.WriteString("🔧 Available OpenTelemetry Instrumentations:\n")
		b.WriteString(strings.Repeat("-", 43))
		b.WriteString("\n")
		for _, instr := range allInstrumentations {
			status := "🔧"
			if instr.IsFirstParty {
				status = "✅"
			}
			fmt.Fprintf(&b, "  %s %s (%s)\n", status, instr.Package.Name, instr.Language)
			if instr.Title != "" && instr.Title != instr.Package.Name {
				fmt.Fprintf(&b, "    📝 %s\n", instr.Title)
			}
			if detailed && instr.Description != "" {
				fmt.Fprintf(&b, "    %s %s\n", "💬", instr.Description)
			}
			if detailed && len(instr.Tags) > 0 {
				fmt.Fprintf(&b, "    %s %s\n", "🏷️ ", strings.Join(instr.Tags, ", "))
			}
		}
		b.WriteString("\n")
	}

	// Issues
	if len(allIssues) > 0 {
		b.WriteString("⚠️  Issues and Recommendations:\n")
		b.WriteString(strings.Repeat("-", 31))
		b.WriteString("\n")

		if len(analysis.DirectoryAnalyses) <= 1 {
			fmt.Fprintf(&b, "Total Issues Found: %d\n\n", len(allIssues))
			for _, issue := range allIssues {
				fmt.Fprintf(&b, "  • %s (%s)\n", issue.Title, issue.Severity)
				if issue.Description != "" {
					fmt.Fprintf(&b, "    %s %s\n", "📖", issue.Description)
				}
				if issue.Suggestion != "" {
					fmt.Fprintf(&b, "    %s %s\n", "💡", issue.Suggestion)
				}
				if detailed && len(issue.References) > 0 {
					fmt.Fprintf(&b, "    %s %s\n", "📚 References:", strings.Join(issue.References, ", "))
				}
				if detailed && issue.File != "" {
					fmt.Fprintf(&b, "    %s %s, Line: %d\n", "📄 File:", issue.File, issue.Line)
				}
				b.WriteString("\n")
			}
		} else {
			// Sort directories by number of issues (desc), then by name
			type dirIssues struct {
				name, language string
				issues         int
			}
			var dirs []dirIssues
			for directory, dirAnalysis := range analysis.DirectoryAnalyses {
				dirs = append(dirs, dirIssues{name: directory, language: dirAnalysis.Language, issues: len(dirAnalysis.Issues)})
			}
			sort.Slice(dirs, func(i, j int) bool {
				if dirs[i].issues == dirs[j].issues {
					return dirs[i].name < dirs[j].name
				}
				return dirs[i].issues > dirs[j].issues
			})

			totalIssues := 0
			for _, di := range dirs {
				dirAnalysis := analysis.DirectoryAnalyses[di.name]
				if len(dirAnalysis.Issues) == 0 {
					continue
				}
				fmt.Fprintf(&b, "📂 %s (%s) — %d issue(s)\n", di.name, di.language, len(dirAnalysis.Issues))
				for _, issue := range dirAnalysis.Issues {
					fmt.Fprintf(&b, "  • %s (%s)\n", issue.Title, issue.Severity)
					if issue.Description != "" {
						fmt.Fprintf(&b, "    %s %s\n", "📖", issue.Description)
					}
					if issue.Suggestion != "" {
						fmt.Fprintf(&b, "    %s %s\n", "💡", issue.Suggestion)
					}
					if detailed && len(issue.References) > 0 {
						fmt.Fprintf(&b, "    %s %s\n", "📚 References:", strings.Join(issue.References, ", "))
					}
					if detailed && issue.File != "" {
						fmt.Fprintf(&b, "    %s %s, Line: %d\n", "📄 File:", issue.File, issue.Line)
					}
					b.WriteString("\n")
					totalIssues++
				}
			}
			fmt.Fprintf(&b, "Total Issues Found: %d\n", totalIssues)
		}
	} else {
		b.WriteString("✅ No issues found! Your OpenTelemetry setup looks good.")
		b.WriteString("\n")
	}

	return b.String()
}
