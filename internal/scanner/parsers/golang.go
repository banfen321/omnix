package parsers

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type GoParser struct{}

func (p *GoParser) Name() string { return "go" }

func (p *GoParser) Detect(dir string, files []string) bool {
	if hasFile(files, "go.mod") {
		return true
	}
	// Fallback
	for _, f := range files {
		if strings.HasSuffix(f, ".go") {
			return true
		}
	}
	return false
}

func (p *GoParser) Parse(dir string) ([]Dep, error) {
	deps := []Dep{}

	f, err := os.Open(filepath.Join(dir, "go.mod"))
	if err != nil {
		return []Dep{{Name: "go", Ecosystem: "go"}}, nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "go ") {
			version := strings.TrimPrefix(line, "go ")
			deps = append(deps, Dep{Name: "go", Version: version, Ecosystem: "go"})
			break
		}
	}

	if len(deps) == 0 {
		deps = append(deps, Dep{Name: "go", Ecosystem: "go"})
	}

	// Detect CGo usage by scanning for `import "C"` in .go files
	if usesCgo(dir) {
		deps = append(deps, Dep{Name: "gcc", Ecosystem: "system"})
		deps = append(deps, Dep{Name: "pkg-config", Ecosystem: "system"})
	}

	return deps, nil
}

// usesCgo scans top-level .go files for `import "C"` to detect CGo usage
func usesCgo(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}

		f, err := os.Open(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == `import "C"` || strings.HasPrefix(line, `// #cgo`) {
				f.Close()
				return true
			}
			// Stop scanning after package-level declarations
			if strings.HasPrefix(line, "func ") || strings.HasPrefix(line, "type ") || strings.HasPrefix(line, "var ") {
				break
			}
		}
		f.Close()
	}
	return false
}
