package parsers

import (
	"os"
	"path/filepath"
	"regexp"
)

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
