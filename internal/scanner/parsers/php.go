package parsers

import (
	"encoding/json"
	"os"
	"path/filepath"
)

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
