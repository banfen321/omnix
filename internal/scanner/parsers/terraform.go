package parsers

import (
	"os"
	"path/filepath"
	"regexp"
)

type TerraformParser struct{}

func (p *TerraformParser) Name() string { return "terraform" }

func (p *TerraformParser) Detect(dir string, files []string) bool {
	return hasFileWithExt(files, ".tf")
}

func (p *TerraformParser) Parse(dir string) ([]Dep, error) {
	deps := []Dep{
		{Name: "terraform", Ecosystem: "terraform"},
	}

	entries, err := filepath.Glob(filepath.Join(dir, "*.tf"))
	if err != nil {
		return deps, nil
	}

	providerRe := regexp.MustCompile(`source\s*=\s*"([^"]+/([^"]+))"`)
	versionRe := regexp.MustCompile(`version\s*=\s*"([^"]*)"`)

	for _, entry := range entries {
		data, err := os.ReadFile(entry)
		if err != nil {
			continue
		}
		content := string(data)

		providers := providerRe.FindAllStringSubmatch(content, -1)
		versions := versionRe.FindAllStringSubmatch(content, -1)

		for i, pm := range providers {
			version := ""
			if i < len(versions) {
				version = extractExactVersion(versions[i][1])
			}
			deps = append(deps, Dep{
				Name:      pm[2],
				Version:   version,
				Ecosystem: "terraform",
			})
		}
	}

	return deps, nil
}
