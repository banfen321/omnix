package parsers

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type GenericParser struct{}

func (p *GenericParser) Name() string { return "generic" }

func (p *GenericParser) Detect(dir string, files []string) bool {
	return hasFile(files, "Makefile") || hasFile(files, ".tool-versions") || hasFile(files, "ansible.cfg")
}

func (p *GenericParser) Parse(dir string) ([]Dep, error) {
	var deps []Dep

	if d, err := parseToolVersions(filepath.Join(dir, ".tool-versions")); err == nil {
		deps = append(deps, d...)
	}

	if _, err := os.Stat(filepath.Join(dir, "Makefile")); err == nil {
		deps = append(deps, Dep{Name: "gnumake", Ecosystem: "system"})
	}

	if _, err := os.Stat(filepath.Join(dir, "ansible.cfg")); err == nil {
		deps = append(deps, Dep{Name: "ansible", Ecosystem: "python"})
	}

	return deps, nil
}

func parseToolVersions(path string) ([]Dep, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var deps []Dep
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		name := parts[0]
		version := parts[1]

		eco := "system"
		switch name {
		case "python":
			eco = "python"
			name = "python3"
		case "nodejs", "node":
			eco = "node"
			name = "nodejs"
		case "golang":
			eco = "go"
			name = "go"
		case "ruby":
			eco = "ruby"
		case "rust":
			eco = "rust"
			name = "rustc"
		case "java":
			eco = "java"
			name = "jdk"
		case "terraform":
			eco = "terraform"
		case "php":
			eco = "php"
		}

		deps = append(deps, Dep{
			Name:      name,
			Version:   version,
			Ecosystem: eco,
		})
	}

	return deps, nil
}
