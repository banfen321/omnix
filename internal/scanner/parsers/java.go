package parsers

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type JavaParser struct{}

func (p *JavaParser) Name() string { return "java" }

func (p *JavaParser) Detect(dir string, files []string) bool {
	return hasFile(files, "pom.xml") || hasFile(files, "build.gradle") || hasFile(files, "build.gradle.kts")
}

func (p *JavaParser) Parse(dir string) ([]Dep, error) {
	var deps []Dep
	deps = append(deps, Dep{Name: "jdk", Ecosystem: "java"})

	if d, err := parsePomXml(filepath.Join(dir, "pom.xml")); err == nil {
		deps = append(deps, d...)
	}

	if d, err := parseGradle(filepath.Join(dir, "build.gradle")); err == nil {
		deps = append(deps, d...)
	}

	if d, err := parseGradle(filepath.Join(dir, "build.gradle.kts")); err == nil {
		deps = append(deps, d...)
	}

	return dedup(deps), nil
}

func parsePomXml(path string) ([]Dep, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	type PomDep struct {
		GroupId    string `xml:"groupId"`
		ArtifactId string `xml:"artifactId"`
		Version    string `xml:"version"`
	}
	type Pom struct {
		Dependencies struct {
			Dep []PomDep `xml:"dependency"`
		} `xml:"dependencies"`
	}

	var pom Pom
	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil, err
	}

	var deps []Dep
	for _, d := range pom.Dependencies.Dep {
		version := d.Version
		if strings.HasPrefix(version, "${") {
			version = ""
		}
		deps = append(deps, Dep{
			Name:      d.GroupId + ":" + d.ArtifactId,
			Version:   version,
			Ecosystem: "java",
		})
	}

	return deps, nil
}

func parseGradle(path string) ([]Dep, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var deps []Dep
	content := string(data)

	gradleDepRe := regexp.MustCompile(`(?:implementation|api|compileOnly|runtimeOnly|testImplementation)\s*[("']([^:]+):([^:]+):([^"')]+)`)
	matches := gradleDepRe.FindAllStringSubmatch(content, -1)

	for _, m := range matches {
		if len(m) >= 4 {
			deps = append(deps, Dep{
				Name:      m[1] + ":" + m[2],
				Version:   m[3],
				Ecosystem: "java",
			})
		}
	}

	if strings.Contains(content, "maven") {
		deps = append(deps, Dep{Name: "maven", Ecosystem: "java"})
	}
	deps = append(deps, Dep{Name: "gradle", Ecosystem: "java"})

	return deps, nil
}
