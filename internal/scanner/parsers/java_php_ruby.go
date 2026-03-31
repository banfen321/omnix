package parsers

import (
	"encoding/json"
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

type PHPParser struct{}

func (p *PHPParser) Name() string { return "php" }

func (p *PHPParser) Detect(dir string, files []string) bool {
	return hasFile(files, "composer.json")
}

func (p *PHPParser) Parse(dir string) ([]Dep, error) {
	data, err := os.ReadFile(filepath.Join(dir, "composer.json"))
	if err != nil {
		return nil, err
	}

	var composer struct {
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
	}
	if err := json.Unmarshal(data, &composer); err != nil {
		return nil, err
	}

	var deps []Dep
	deps = append(deps, Dep{Name: "php", Ecosystem: "php"})

	for name, ver := range composer.Require {
		if name == "php" {
			deps[0].Version = extractSemver(ver)
			continue
		}
		deps = append(deps, Dep{
			Name:      name,
			Version:   extractSemver(ver),
			Ecosystem: "php",
		})
	}

	return deps, nil
}

type RubyParser struct{}

func (p *RubyParser) Name() string { return "ruby" }

func (p *RubyParser) Detect(dir string, files []string) bool {
	return hasFile(files, "Gemfile")
}

func (p *RubyParser) Parse(dir string) ([]Dep, error) {
	data, err := os.ReadFile(filepath.Join(dir, "Gemfile"))
	if err != nil {
		return nil, err
	}

	var deps []Dep
	deps = append(deps, Dep{Name: "ruby", Ecosystem: "ruby"})

	gemRe := regexp.MustCompile(`gem\s+['"]([^'"]+)['"](?:\s*,\s*['"]([^'"]+)['"])?`)
	matches := gemRe.FindAllStringSubmatch(string(data), -1)

	for _, m := range matches {
		dep := Dep{Name: m[1], Ecosystem: "ruby"}
		if len(m) > 2 && m[2] != "" {
			dep.Version = extractExactVersion(m[2])
		}
		deps = append(deps, dep)
	}

	return deps, nil
}
