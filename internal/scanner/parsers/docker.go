package parsers

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type DockerParser struct{}

func (p *DockerParser) Name() string { return "docker" }

func (p *DockerParser) Detect(dir string, files []string) bool {
	return hasFile(files, "Dockerfile") || hasFile(files, "docker-compose.yml") || hasFile(files, "docker-compose.yaml")
}

func (p *DockerParser) Parse(dir string) ([]Dep, error) {
	var deps []Dep

	if d, err := parseDockerfile(filepath.Join(dir, "Dockerfile")); err == nil {
		deps = append(deps, d...)
	}

	return deps, nil
}

func parseDockerfile(path string) ([]Dep, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var deps []Dep
	fromRe := regexp.MustCompile(`^FROM\s+([^:\s]+)(?::([^\s]+))?`)
	aptRe := regexp.MustCompile(`(?:apt-get install|apt install)\s+(?:-[yq]\s+)*(.+)`)
	pipRe := regexp.MustCompile(`pip(?:3)?\s+install\s+(.+)`)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if m := fromRe.FindStringSubmatch(line); m != nil {
			image := m[1]
			version := ""
			if m[2] != "" && m[2] != "latest" {
				version = m[2]
			}
			if strings.Contains(image, "python") {
				deps = append(deps, Dep{Name: "python3", Version: version, Ecosystem: "python"})
			} else if strings.Contains(image, "node") {
				deps = append(deps, Dep{Name: "nodejs", Version: version, Ecosystem: "node"})
			} else if strings.Contains(image, "golang") || strings.Contains(image, "go") {
				deps = append(deps, Dep{Name: "go", Version: version, Ecosystem: "go"})
			} else if strings.Contains(image, "rust") {
				deps = append(deps, Dep{Name: "rustc", Version: version, Ecosystem: "rust"})
			}
		}

		if m := aptRe.FindStringSubmatch(line); m != nil {
			pkgs := strings.Fields(m[1])
			for _, pkg := range pkgs {
				pkg = strings.TrimSuffix(pkg, "\\")
				pkg = strings.TrimSpace(pkg)
				if pkg != "" && !strings.HasPrefix(pkg, "-") {
					deps = append(deps, Dep{Name: pkg, Ecosystem: "system"})
				}
			}
		}

		if m := pipRe.FindStringSubmatch(line); m != nil {
			pkgs := strings.Fields(m[1])
			for _, pkg := range pkgs {
				pkg = strings.TrimSuffix(pkg, "\\")
				pkg = strings.TrimSpace(pkg)
				if pkg != "" && !strings.HasPrefix(pkg, "-") {
					parts := strings.SplitN(pkg, "==", 2)
					dep := Dep{Name: strings.ToLower(parts[0]), Ecosystem: "python"}
					if len(parts) == 2 {
						dep.Version = parts[1]
					}
					deps = append(deps, dep)
				}
			}
		}
	}

	return deps, nil
}
