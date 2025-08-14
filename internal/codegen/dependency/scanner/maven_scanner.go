package scanner

import (
	"encoding/xml"
	"os"
	"path/filepath"
)

// MavenScanner scans pom.xml for Java dependencies
type MavenScanner struct{}

// NewMavenScanner creates a new Maven scanner
func NewMavenScanner() Scanner {
	return &MavenScanner{}
}

// Detect checks for pom.xml
func (s *MavenScanner) Detect(projectPath string) bool {
	_, err := os.Stat(filepath.Join(projectPath, "pom.xml"))
	return err == nil
}

// Scan reads pom.xml and returns dependency coordinates
func (s *MavenScanner) Scan(projectPath string) ([]string, error) {
	file, err := os.ReadFile(filepath.Join(projectPath, "pom.xml"))
	if err != nil {
		return nil, err
	}

	var pom struct {
		Dependencies struct {
			Dependency []struct {
				GroupId    string `xml:"groupId"`
				ArtifactId string `xml:"artifactId"`
			} `xml:"dependency"`
		} `xml:"dependencies"`
	}

	if err := xml.Unmarshal(file, &pom); err != nil {
		return nil, err
	}

	var deps []string
	for _, dep := range pom.Dependencies.Dependency {
		if dep.GroupId != "" && dep.ArtifactId != "" {
			deps = append(deps, dep.GroupId+":"+dep.ArtifactId)
		}
	}

	return deps, nil
}
